package domain

import (
	"StreamRoom/internal/views"
	"encoding/json"

	"github.com/gorilla/websocket"
)

// VideoStatus tracks a room's video through the upload/transcode pipeline.
type VideoStatus string

const (
	VideoStatusPending    VideoStatus = "PENDING"    // upload URL issued, nothing received yet
	VideoStatusProcessing VideoStatus = "PROCESSING" // upload confirmed, worker is transcoding
	VideoStatusReady      VideoStatus = "READY"      // HLS artifacts live on the CDN origin
	VideoStatusFailed     VideoStatus = "FAILED"     // exhausted retries; see VideoError
)

// Broadcast sends a JSON payload to every currently connected client in the
// room, dropping (and cleaning up) any client whose write fails. Shared by
// the periodic sync ticker and one-off events like a video becoming ready.
func (r *Room) Broadcast(payload []byte) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	for client := range r.Clients {
		if err := client.Conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			client.Conn.Close()
			delete(r.Clients, client)
		}
	}
}

// SetVideoProcessing marks the room's video as actively transcoding and
// notifies connected clients so the UI can show a progress state.
func (r *Room) SetVideoProcessing() {
	r.Mu.Lock()
	r.VideoStatus = VideoStatusProcessing
	r.VideoError = ""
	r.Mu.Unlock()

	r.broadcastVideoStatus(views.VideoStatusMessage{
		Type:   "VIDEO_STATUS",
		Status: string(VideoStatusProcessing),
	})
}

// SetVideoReady records the finished HLS master playlist + available
// renditions and notifies connected clients that playback can start.
func (r *Room) SetVideoReady(masterPlaylistURL string, qualities []string) {
	r.Mu.Lock()
	r.VideoStatus = VideoStatusReady
	r.PlaybackURL = masterPlaylistURL
	r.Qualities = qualities
	r.VideoError = ""
	r.Mu.Unlock()

	r.broadcastVideoStatus(views.VideoStatusMessage{
		Type:           "VIDEO_STATUS",
		Status:         string(VideoStatusReady),
		MasterPlaylist: masterPlaylistURL,
		Qualities:      qualities,
	})
}

// SetVideoFailed records a terminal (retries exhausted) processing failure
// and notifies connected clients.
func (r *Room) SetVideoFailed(reason string) {
	r.Mu.Lock()
	r.VideoStatus = VideoStatusFailed
	r.VideoError = reason
	r.Mu.Unlock()

	r.broadcastVideoStatus(views.VideoStatusMessage{
		Type:   "VIDEO_STATUS",
		Status: string(VideoStatusFailed),
		Error:  reason,
	})
}

func (r *Room) broadcastVideoStatus(msg views.VideoStatusMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	r.Broadcast(payload)
}
