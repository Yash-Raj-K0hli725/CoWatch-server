package service

import (
	"StreamRoom/internal/domain"
	"StreamRoom/internal/views"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RoomService handles generation and storage of synced rooms
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
	roomID := fmt.Sprintf("CO-WATCH-%d", time.Now().UnixNano()%100000)
	obzectKey := fmt.Sprintf("videos/%s/%s_%d.mp4", roomID, roomID, time.Now().Unix()/1000)
	uploadURL, err := s.v.GenerateUploadUrl(c, obzectKey)
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
		Clients:           make(map[*domain.Client]struct{}),
		IsPlaying:         true, // Play by default when room initializes
		CurrentPositionMs: 0,
		Obzect:            obzectKey,
		LastUpdated:       time.Now(),
		Ctx:               ctx,
		Cancel:            cancel,
	}

	return newRoom, nil
}

func (s *RoomService) Konnection(room *domain.Room, client *domain.Client) {
	defer func() {
		room.Synx.Lock()
		delete(room.Clients, client)
		_ = client.Conn.Close()
		room.Synx.Unlock()
	}()

	room.Synx.Lock()
	room.Clients[client] = struct{}{}
	room.Synx.Unlock()
	// ACTIVE READ LOOP: Listen for incoming Pause/Play/Seek events from this client
	for {
		_, msgBytes, err := client.Conn.ReadMessage()
		if err != nil {
			break // Client disconnected
		}

		var action views.ActionRequest
		if err := json.Unmarshal(msgBytes, &action); err == nil {
			room.HandleAction(action)
		}
	}
}
