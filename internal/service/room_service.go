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
type RoomService struct{}

func NewRoomService() *RoomService {
	return &RoomService{}
}

func (s *RoomService) GetCreateRoom(request views.CreateRoomRequest) *views.RoomResponse {
	domain.RoomsMu.Lock()
	defer domain.RoomsMu.Unlock()

	// Generate a short, unique alphanumeric room code
	roomID := fmt.Sprintf("ROOM-%d", time.Now().UnixNano()%100000)

	newRoom := &views.RoomResponse{
		ID:        roomID,
		RoomName:  request.RoomName,
		StartedAt: time.Now(), // The live broadcast ticker clock begins ticking NOW
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
	}

	// Note: In your final system, you would trigger the background synchronization loop
	// (like the ticker we designed earlier) right here using: go runRoomSyncLoop(newRoom)

	return newRoom
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
