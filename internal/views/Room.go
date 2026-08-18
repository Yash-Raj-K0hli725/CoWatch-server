package views

import (
	"time"
)

// RoomResponse representation
type RoomResponse struct {
	ID        string    `json:"room_id"`
	RoomName  string    `json:"room_name"`
	UploadUrl string    `json:"upload_url"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateRoomRequest struct {
	RoomName string `json:"room_name"`
}

// RoomStatusResponse is the public shape returned by GET /room/find. It is
// hand-built from domain.Room field-by-field rather than serializing the
// domain struct directly, which otherwise leaks internal state (bucket
// object keys, the live websocket client set, sync primitives, contexts)
// to any caller who can guess a room ID.
type RoomStatusResponse struct {
	ID                string   `json:"room_id"`
	IsPlaying         bool     `json:"is_playing"`
	CurrentPositionMs int64    `json:"position_ms"`
	VideoStatus       string   `json:"video_status"`
	PlaybackURL       string   `json:"playback_url,omitempty"`
	Qualities         []string `json:"qualities,omitempty"`
	VideoError        string   `json:"video_error,omitempty"`
}
