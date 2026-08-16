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
