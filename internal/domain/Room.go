package domain

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Conn *websocket.Conn
}

type Room struct {
	ID      string
	Clients map[*Client]bool
	Mu      sync.Mutex
	Ctx     context.Context
	Cancel  context.CancelFunc

	// Shared Video State (playback sync)
	IsPlaying         bool
	CurrentPositionMs int64 // The anchored playback time
	LastUpdated       time.Time

	// Upload/transcode pipeline state
	SourceObjectKey string // bucket key the client's presigned PUT targets
	// ActiveJobID is the JobID of the transcode job this room is currently
	// waiting on. Video lifecycle events are only applied if they carry a
	// matching JobID -- this is what stops a superseded job (e.g. the
	// video was replaced via RegenerateUploadUrl while an old transcode
	// was still running) from finishing late and stomping the room's
	// current state with stale data.
	ActiveJobID string
	VideoStatus VideoStatus // PENDING -> PROCESSING -> READY|FAILED
	PlaybackURL string      // CDN URL of master.m3u8, once READY
	Qualities   []string    // e.g. ["1080p","720p","480p","360p"], once READY
	VideoError  string      // last failure reason, once FAILED
}

// GetLivePosition calculates the real-time position safely
func (r *Room) GetLivePosition() int64 {
	if !r.IsPlaying {
		return r.CurrentPositionMs
	}
	// If playing, position is anchor position + time elapsed since last anchor change
	return r.CurrentPositionMs + time.Since(r.LastUpdated).Milliseconds()
}
