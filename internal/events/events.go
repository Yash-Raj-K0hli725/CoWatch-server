// Package events carries video-pipeline lifecycle notifications
// (processing started / ready / failed) from the worker fleet back to
// whichever API server process is holding the room's websocket
// connections, over Redis Pub/Sub. It deliberately knows nothing about
// rooms or websockets -- that's the API server's job on the receiving end.
package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

// Channel is the single shared Pub/Sub channel every API server instance
// subscribes to. Fan-out is Redis's job: every subscriber gets every
// message, and each subscriber only acts on events for rooms it actually
// holds (domain.GetRoom returns nil otherwise), so this scales to N API
// replicas without any extra routing logic.
const Channel = "streamroom:video-events"

type VideoStatus string

const (
	VideoStatusProcessing VideoStatus = "PROCESSING"
	VideoStatusReady      VideoStatus = "READY"
	VideoStatusFailed     VideoStatus = "FAILED"
)

// VideoEvent describes a change in a room's video processing lifecycle.
type VideoEvent struct {
	RoomID         string      `json:"room_id"`
	JobID          string      `json:"job_id"`
	Status         VideoStatus `json:"status"`
	MasterPlaylist string      `json:"master_playlist_url,omitempty"`
	Qualities      []string    `json:"qualities,omitempty"`
	Error          string      `json:"error,omitempty"`
}

// Publisher is used by the worker fleet to announce job outcomes.
type Publisher struct {
	rdb *redis.Client
}

func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

func (p *Publisher) Publish(ctx context.Context, e VideoEvent) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, Channel, payload).Err()
}

// Handler reacts to a decoded VideoEvent.
type Handler func(VideoEvent)

// Subscribe starts a background goroutine forwarding decoded events on
// Channel to handler until ctx is cancelled. Used by the API server to
// bridge worker-published events back into in-memory room/websocket state.
func Subscribe(ctx context.Context, rdb *redis.Client, handler Handler) {
	sub := rdb.Subscribe(ctx, Channel)
	ch := sub.Channel()
	go func() {
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var e VideoEvent
				if err := json.Unmarshal([]byte(msg.Payload), &e); err != nil {
					log.Printf("events: dropping unparseable message on %s: %v", Channel, err)
					continue
				}
				handler(e)
			}
		}
	}()
}
