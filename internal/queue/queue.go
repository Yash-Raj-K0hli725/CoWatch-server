// Package queue defines a broker-agnostic contract for the transcode job
// pipeline. The API-side handlers only ever see the Producer interface, and
// the worker fleet only ever sees the Consumer interface, so the underlying
// broker (Redis Streams today; SQS/RabbitMQ if we outgrow it) can change
// without touching either side's business logic.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrDuplicateJob is returned by Producer.Enqueue when the same object
// version (bucket + key + etag) has already been enqueued within the
// dedupe window. Callers should treat it as a success, not a failure --
// it means the work is already in flight or done, which is exactly what
// double-delivered bucket events / retried client confirmations produce.
var ErrDuplicateJob = errors.New("queue: job already enqueued for this object version")

// TranscodeJob is the unit of work handed from the upload-ingest path
// (webhook or client-confirm) to the Go worker fleet.
type TranscodeJob struct {
	JobID       string    `json:"job_id"`
	RoomID      string    `json:"room_id"`
	Bucket      string    `json:"bucket"`
	ObjectKey   string    `json:"object_key"`
	ContentType string    `json:"content_type,omitempty"`
	SizeBytes   int64     `json:"size_bytes,omitempty"`
	ETag        string    `json:"etag,omitempty"`
	Source      string    `json:"source"` // "webhook" | "client-confirm"
	EnqueuedAt  time.Time `json:"enqueued_at"`
	// Attempt is best-effort delivery count, populated by the queue on
	// reclaim; workers may use it to tune timeouts/logging but must not
	// rely on it for correctness (at-least-once, not exactly-once).
	Attempt int `json:"attempt"`
}

func (j *TranscodeJob) Marshal() ([]byte, error) { return json.Marshal(j) }

// Delivery wraps a dequeued job together with the broker-native handle
// needed to Ack/Nack it.
type Delivery struct {
	Job Job
	ID  string
	// LastError is only populated on deliveries returned from the
	// dead-letter side of Reclaim, carrying the last recorded failure
	// reason for that job.
	LastError string
}

// Job is an alias kept for readability at call sites (Delivery.Job.RoomID
// reads oddly otherwise); it's the same type as TranscodeJob.
type Job = TranscodeJob

// Producer enqueues jobs. Implementations must be safe for concurrent use.
//
// job is passed by pointer because a duplicate-detected Enqueue (see
// ErrDuplicateJob) mutates job.JobID to whichever JobID actually owns the
// dedupe key -- otherwise a caller that generated its own JobID up front
// (to guard against a job finishing before it's recorded, e.g.
// RoomService.ActiveJobID) would keep believing a fake, never-enqueued ID
// is the one in flight.
type Producer interface {
	Enqueue(ctx context.Context, job *TranscodeJob) error
}

// Consumer pulls jobs for processing with an at-least-once, consumer-group
// style contract: a delivered job stays "pending" in the broker until Ack'd.
// A job that's neither Ack'd nor re-claimed simply sits pending -- Reclaim
// is what turns idle pending entries back into deliverable work (or, past
// maxDeliveries, into a dead-letter entry).
type Consumer interface {
	// Consume blocks pulling up to count new deliveries, waiting up to
	// block for messages to arrive if none are immediately available.
	Consume(ctx context.Context, count int64, block time.Duration) ([]Delivery, error)
	// Reclaim scans for deliveries idle longer than minIdle. Ones that
	// have already been delivered maxDeliveries times are dead-lettered
	// (returned in deadLettered, already Ack'd off the main stream);
	// everything else is claimed for this consumer and returned in
	// reclaimed so the caller can retry it.
	Reclaim(ctx context.Context, minIdle time.Duration, maxDeliveries int64) (reclaimed []Delivery, deadLettered []Delivery, err error)
	Ack(ctx context.Context, id string) error
	// Nack records that this attempt failed. It intentionally does not
	// requeue immediately -- the delivery stays pending and Reclaim picks
	// it back up once minIdle has elapsed, giving transient failures
	// (a network blip, a momentarily unavailable bucket) a cool-down
	// instead of hammering the same failure in a tight loop.
	Nack(ctx context.Context, id string, reason error) error
}
