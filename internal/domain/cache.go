package domain

import (
	"sync"
)

// Global In-Memory Database for demonstration
var (
	RoomsMap = make(map[string]*Room)
	RoomsMu  sync.RWMutex
)

func DeleteRoom(id string) {
	RoomsMu.Lock()
	defer RoomsMu.Unlock()
	delete(RoomsMap, id)
}

// GetRoom is the only sanctioned way to read RoomsMap -- it's called from
// HTTP handlers, the websocket join path, and (now) the video-events
// listener on every pub/sub message for the server's whole lifetime, all
// concurrently with RoomService.GetCreateRoom's writes and the per-room
// ticker's cleanup deletes. Bypassing this (e.g. indexing RoomsMap
// directly) is a "concurrent map read and map write" away from crashing
// the process.
func GetRoom(id string) *Room {
	RoomsMu.RLock()
	defer RoomsMu.RUnlock()
	return RoomsMap[id]
}
