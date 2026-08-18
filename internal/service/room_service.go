package service

import (
	"StreamRoom/errz"
	"StreamRoom/internal/domain"
	"StreamRoom/internal/events"
	"StreamRoom/internal/queue"
	"StreamRoom/internal/views"
	"StreamRoom/util"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/redis/go-redis/v9"
)

// RoomService handles generation and storage of synced rooms, plus
// coordinating the upload -> transcode -> "video ready" pipeline for each
// room's video.
type RoomService struct {
	v *VideoService
}

func NewRoomService(v *VideoService) *RoomService {
	return &RoomService{v: v}
}

func (s *RoomService) GetCreateRoom(c context.Context, request views.CreateRoomRequest) (*views.RoomResponse, error) {
	domain.RoomsMu.Lock()
	defer domain.RoomsMu.Unlock()

	// Generate a short, unique alphanumeric room code
	roomID := fmt.Sprintf("ROOM-%d", time.Now().UnixNano()%100000)
	uploadURL, objectKey, err := s.v.GenerateUploadUrl(c, roomID)
	if err != nil {
		return nil, err
	}
	newRoom := &views.RoomResponse{
		ID:        roomID,
		UploadUrl: uploadURL,
		RoomName:  request.RoomName,
		CreatedAt: time.Now(), // The live broadcast ticker clock begins ticking NOW
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	domain.RoomsMap[roomID] = &domain.Room{
		ID:                roomID,
		Clients:           make(map[*domain.Client]bool),
		IsPlaying:         true, // Play by default when room initializes
		CurrentPositionMs: 0,
		LastUpdated:       time.Now(),
		Ctx:               ctx,
		Cancel:            cancel,
		SourceObjectKey:   objectKey,
		VideoStatus:       domain.VideoStatusPending,
	}

	// Note: In your final system, you would trigger the background synchronization loop
	// (like the ticker we designed earlier) right here using: go runRoomSyncLoop(newRoom)

	return newRoom, nil
}

func (s *RoomService) Konnection(room *domain.Room, client *domain.Client) {
	defer func() {
		room.Mu.Lock()
		delete(room.Clients, client)
		client.Conn.Close()
		room.Mu.Unlock()
	}()

	room.Mu.Lock()
	room.Clients[client] = true
	room.Mu.Unlock()
	// ACTIVE READ LOOP: Listen for incoming Pause/Play/Seek events from this client
	for {
		_, msgBytes, err := client.Conn.ReadMessage()
		if err != nil {
			break // Client disconnected
		}

		var actionMsg views.ActionRequest
		if err := json.Unmarshal(msgBytes, &actionMsg); err == nil {
			room.HandleAction(actionMsg)
		}
	}
}

// RegenerateUploadUrl issues a fresh presigned upload URL for an existing
// room (initial upload, or replacing the video) and resets its video
// pipeline state back to PENDING.
func (s *RoomService) RegenerateUploadUrl(ctx context.Context, roomID string) (*views.UploadURLResponse, error) {
	room := domain.GetRoom(roomID)
	if room == nil {
		return nil, errz.NewNotFound("room not found")
	}

	uploadURL, objectKey, err := s.v.GenerateUploadUrl(ctx, roomID)
	if err != nil {
		return nil, err
	}

	room.Mu.Lock()
	room.SourceObjectKey = objectKey
	// No transcode job exists for the new upload yet -- clearing
	// ActiveJobID means a still-running job for the *previous* video (the
	// one just replaced) can no longer match on completion and will be
	// ignored by StartVideoEventListener instead of stomping this state.
	room.ActiveJobID = ""
	room.VideoStatus = domain.VideoStatusPending
	room.PlaybackURL = ""
	room.Qualities = nil
	room.VideoError = ""
	room.Mu.Unlock()

	return &views.UploadURLResponse{UploadURL: uploadURL, ObjectKey: objectKey}, nil
}

// ConfirmVideoUpload is called after the client's presigned PUT succeeds.
// It never trusts the object key the client would otherwise have to send --
// it looks up the key *this room* was issued and hands that to VideoService,
// which independently verifies the object exists before enqueueing.
func (s *RoomService) ConfirmVideoUpload(ctx context.Context, roomID string) (*queue.TranscodeJob, error) {
	room := domain.GetRoom(roomID)
	if room == nil {
		return nil, errz.NewNotFound("room not found")
	}

	room.Mu.Lock()
	objectKey := room.SourceObjectKey
	room.Mu.Unlock()
	if objectKey == "" {
		return nil, errz.NewBadRequest("room has no pending upload")
	}

	// JobID is generated here (not inside VideoService) and recorded as
	// this room's ActiveJobID *before* the job is enqueued, so even a
	// worker that picks the job up and publishes its outcome instantly
	// can't race ahead of us -- StartVideoEventListener only applies
	// events whose JobID matches ActiveJobID.
	jobID := util.NewID("job")
	room.Mu.Lock()
	room.ActiveJobID = jobID
	room.Mu.Unlock()

	job, err := s.v.ConfirmUpload(ctx, roomID, objectKey, jobID)
	if err != nil {
		return nil, err
	}
	// A duplicate-detected enqueue may have corrected job.JobID to
	// whichever job actually owns this object version; re-record it so
	// ActiveJobID matches the job that's really in flight.
	room.Mu.Lock()
	room.ActiveJobID = job.JobID
	room.Mu.Unlock()
	room.SetVideoProcessing()
	return job, nil
}

// IngestVideoUploadEvent handles a verified storage webhook event. The
// event carries a bucket + object key (no room ID), so the room is
// recovered by parsing the "uploads/<roomID>/<file>" key layout.
func (s *RoomService) IngestVideoUploadEvent(ctx context.Context, bucket, objectKey string, sizeBytes int64, etag string) (*queue.TranscodeJob, error) {
	roomID, err := util.ParseRoomIDFromObjectKey(objectKey)
	if err != nil {
		return nil, errz.NewBadRequest(err.Error())
	}

	room := domain.GetRoom(roomID)

	jobID := util.NewID("job")
	if room != nil {
		room.Mu.Lock()
		room.ActiveJobID = jobID
		room.Mu.Unlock()
	}

	job, err := s.v.IngestStorageEvent(ctx, roomID, bucket, objectKey, jobID, sizeBytes, etag)
	if err != nil {
		return nil, err
	}

	if room != nil {
		room.Mu.Lock()
		room.ActiveJobID = job.JobID
		room.Mu.Unlock()
		room.SetVideoProcessing()
	}
	// A nil room (e.g. it expired/was cleaned up between upload and event
	// delivery) is not an error: the job still runs, and the "ready" event
	// will simply have no room left to notify.
	return job, nil
}

// StartVideoEventListener subscribes to the shared Redis pub/sub channel
// the worker fleet publishes job outcomes on, and applies them to whichever
// room this API server instance is holding in memory. Safe to call once at
// server startup; it runs until ctx is cancelled.
func (s *RoomService) StartVideoEventListener(ctx context.Context, rdb *redis.Client) {
	events.Subscribe(ctx, rdb, func(e events.VideoEvent) {
		room := domain.GetRoom(e.RoomID)
		if room == nil {
			log.Infof("video-events: room %s not held by this instance, ignoring %s event for job %s", e.RoomID, e.Status, e.JobID)
			return
		}

		room.Mu.Lock()
		active := room.ActiveJobID
		room.Mu.Unlock()
		if active != e.JobID {
			// This event belongs to a job the room has since moved past
			// (the video was replaced, or another job already won) --
			// applying it would silently overwrite newer state with
			// stale data.
			log.Infof("video-events: room %s ignoring stale %s event for job %s (active job is %s)", e.RoomID, e.Status, e.JobID, active)
			return
		}

		switch e.Status {
		case events.VideoStatusReady:
			room.SetVideoReady(e.MasterPlaylist, e.Qualities)
		case events.VideoStatusFailed:
			room.SetVideoFailed(e.Error)
		case events.VideoStatusProcessing:
			room.SetVideoProcessing()
		default:
			log.Warnf("video-events: room %s received unrecognized video status %q for job %s", e.RoomID, e.Status, e.JobID)
		}
	})
}
