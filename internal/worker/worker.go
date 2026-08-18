package worker

import (
	"context"
	"log"
	"time"

	"StreamRoom/internal/events"
	"StreamRoom/internal/queue"
)

// Worker pulls TranscodeJobs off a queue.Consumer and runs them through a
// Processor with bounded concurrency, publishing the outcome of every job
// (success or terminal failure) as a events.VideoEvent for the API/
// websocket server to relay to the room.
type Worker struct {
	consumer  queue.Consumer
	processor *Processor
	publisher *events.Publisher
	cfg       Config
}

func New(consumer queue.Consumer, processor *Processor, publisher *events.Publisher, cfg Config) *Worker {
	return &Worker{consumer: consumer, processor: processor, publisher: publisher, cfg: cfg}
}

// Run fans deliveries out across cfg.Concurrency worker goroutines and
// runs a background reclaim loop, until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	jobs := make(chan queue.Delivery)
	defer close(jobs)

	for i := 0; i < w.cfg.Concurrency; i++ {
		go w.runLoop(ctx, jobs)
	}
	go w.reclaimLoop(ctx, jobs)

	for {
		if ctx.Err() != nil {
			return
		}

		deliveries, err := w.consumer.Consume(ctx, int64(w.cfg.Concurrency), 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("worker: consume error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _, d := range deliveries {
			select {
			case jobs <- d:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Worker) runLoop(ctx context.Context, jobs <-chan queue.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-jobs:
			if !ok {
				return
			}
			w.process(ctx, d)
		}
	}
}

// reclaimLoop periodically scans for abandoned/failed deliveries and feeds
// anything reclaimable back into the same bounded jobs channel the fresh-
// delivery loop uses -- never processed inline here, so a slow reclaimed
// job can't stall the reclaim ticker itself or bypass cfg.Concurrency.
func (w *Worker) reclaimLoop(ctx context.Context, jobs chan<- queue.Delivery) {
	interval := w.cfg.ReclaimInterval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reclaimed, deadLettered, err := w.consumer.Reclaim(ctx, w.cfg.VisibilityTimeout, w.cfg.MaxDeliveries)
			if err != nil {
				log.Printf("worker: reclaim error: %v", err)
				continue
			}

			for _, d := range deadLettered {
				log.Printf("worker: job %s (room %s) exceeded max deliveries, dead-lettered; last error: %s", d.Job.JobID, d.Job.RoomID, d.LastError)
				w.publish(ctx, events.VideoEvent{
					RoomID: d.Job.RoomID,
					JobID:  d.Job.JobID,
					Status: events.VideoStatusFailed,
					Error:  d.LastError,
				})
			}

			for _, d := range reclaimed {
				log.Printf("worker: reclaimed abandoned/failed job %s (room %s), attempt %d", d.Job.JobID, d.Job.RoomID, d.Job.Attempt)
				select {
				case jobs <- d:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// process runs one delivery through the pipeline and reacts to the
// outcome. A failure is Nack'd (not immediately retried -- see
// queue.Consumer.Nack) and does NOT publish a Failed event: retries are
// silent to the room until either the job succeeds or reclaimLoop
// eventually dead-letters it for good.
func (w *Worker) process(ctx context.Context, d queue.Delivery) {
	log.Printf("worker: processing job %s (room %s, key %s, attempt %d)", d.Job.JobID, d.Job.RoomID, d.Job.ObjectKey, d.Job.Attempt)

	result, err := w.processor.Process(ctx, d.Job)
	if err != nil {
		log.Printf("worker: job %s failed (attempt %d): %v", d.Job.JobID, d.Job.Attempt, err)
		if nackErr := w.consumer.Nack(ctx, d.ID, err); nackErr != nil {
			log.Printf("worker: failed to record nack for job %s: %v", d.Job.JobID, nackErr)
		}
		return
	}

	if err := w.consumer.Ack(ctx, d.ID); err != nil {
		log.Printf("worker: failed to ack job %s: %v", d.Job.JobID, err)
	}

	log.Printf("worker: job %s (room %s) ready: %s", d.Job.JobID, d.Job.RoomID, result.MasterPlaylistURL)
	w.publish(ctx, events.VideoEvent{
		RoomID:         d.Job.RoomID,
		JobID:          d.Job.JobID,
		Status:         events.VideoStatusReady,
		MasterPlaylist: result.MasterPlaylistURL,
		Qualities:      result.Qualities,
	})
}

func (w *Worker) publish(ctx context.Context, e events.VideoEvent) {
	if w.publisher == nil {
		return
	}
	if err := w.publisher.Publish(ctx, e); err != nil {
		log.Printf("worker: failed to publish %s event for job %s: %v", e.Status, e.JobID, err)
	}
}
