package domain

import (
	"StreamRoom/internal/views"
	"StreamRoom/util/enums"
	"encoding/json"
	"time"

	"github.com/labstack/gommon/log"
)

func (r *Room) startBroadcasting() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.Mu.Lock()
			empty := len(r.Clients) == 0
			msg := views.SyncResponse{
				IsPlaying: r.IsPlaying,
				CurrentMS: r.GetLivePosition(),
			}
			r.Mu.Unlock()

			if empty {
				log.Printf("[Room %s] Cleaning up empty room", r.ID)
				DeleteRoom(r.ID)
				r.Cancel()
				return
			}

			payload, _ := json.Marshal(msg)
			r.Broadcast(payload)

		case <-r.Ctx.Done():
			return
		}
	}
}

func (r *Room) HandleAction(act views.ActionRequest) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	do := enums.Action(act.Action)
	switch do {
	case enums.PAUSE:
		if r.IsPlaying {
			// Anchor the exact position where it paused
			r.CurrentPositionMs = r.GetLivePosition()
			r.IsPlaying = false
			r.LastUpdated = time.Now()
			log.Infof("[Room %s] Video PAUSED at %d ms", r.ID, r.CurrentPositionMs)
		}

	case enums.PLAY:
		if !r.IsPlaying {
			// Resume timeline from current anchor position
			r.IsPlaying = true
			r.LastUpdated = time.Now()
			log.Infof("[Room %s] Video PLAYED from %d ms", r.ID, r.CurrentPositionMs)
		}

	case enums.SEEK:
		// Force new timestamp anchor point
		r.CurrentPositionMs = act.PositionMs
		r.LastUpdated = time.Now()
		log.Infof("[Room %s] Video SEEKED to %d ms", r.ID, act.PositionMs)
	}
}
