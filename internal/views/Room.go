package views

import (
	"time"
)

// RoomResponse representation
type RoomResponse struct {
	ID        string    `json:"room_id"`
	VideoURL  string    `json:"video_url"`
	RoomName  string    `json:"room_name"`
	StartedAt time.Time `json:"started_at"`
}

type CreateRoomRequest struct {
	RoomName string `json:"room_name"`
}
