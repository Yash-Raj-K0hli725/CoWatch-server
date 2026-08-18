package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// dedupeTTL bounds how long we remember "this object version was already
// enqueued" -- long enough to absorb duplicate bucket-event deliveries and
// retried client-confirm calls, short enough not to leak keys forever.
const dedupeTTL = 24 * time.Hour

// RedisStream implements Producer and Consumer on top of a single Redis
// Stream + consumer group. Multiple worker processes can share the same
// (stream, group) with distinct consumerName values to horizontally scale
// out -- Redis hands each pending message to exactly one consumer at a time.
type RedisStream struct {
	rdb          *redis.Client
	stream       string
	deadLetter   string
	group        string
	consumerName string
}

func NewRedisStream(rdb *redis.Client, stream, group, consumerName string) *RedisStream {
	return &RedisStream{
		rdb:          rdb,
		stream:       stream,
		deadLetter:   stream + ":dlq",
		group:        group,
		consumerName: consumerName,
	}
}

// EnsureGroup creates the stream + consumer group if they don't already
// exist. Safe (and expected) to call on every process start -- a
// pre-existing group is left untouched, including its position and pending
// entries, so restarts don't lose or replay history.
func (q *RedisStream) EnsureGroup(ctx context.Context) error {
	err := q.rdb.XGroupCreateMkStream(ctx, q.stream, q.group, "$").Err()
	if err != nil && !isBusyGroupErr(err) {
		return fmt.Errorf("create consumer group %q on stream %q: %w", q.group, q.stream, err)
	}
	return nil
}

func isBusyGroupErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

// Enqueue is idempotent per (bucket, object_key, etag): if that exact
// object version has already been enqueued within dedupeTTL, this returns
// ErrDuplicateJob instead of adding a second entry. That's the guard
// against duplicate bucket-event deliveries and a client retrying its
// upload-complete call.
func (q *RedisStream) Enqueue(ctx context.Context, job *TranscodeJob) error {
	dedupeKey := fmt.Sprintf("streamroom:dedupe:%s:%s:%s", job.Bucket, job.ObjectKey, job.ETag)
	ok, err := q.rdb.SetNX(ctx, dedupeKey, job.JobID, dedupeTTL).Result()
	if err != nil {
		return fmt.Errorf("dedupe check: %w", err)
	}
	if !ok {
		// Someone already owns this object version's job. Correct
		// job.JobID to theirs so the caller reports/tracks the job that's
		// actually in flight, not the one it speculatively generated.
		if existing, err := q.rdb.Get(ctx, dedupeKey).Result(); err == nil && existing != "" {
			job.JobID = existing
		}
		return ErrDuplicateJob
	}

	payload, err := job.Marshal()
	if err != nil {
		q.rdb.Del(ctx, dedupeKey)
		return fmt.Errorf("marshal job: %w", err)
	}

	if err := q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]interface{}{"job": payload},
	}).Err(); err != nil {
		q.rdb.Del(ctx, dedupeKey)
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}

func (q *RedisStream) Consume(ctx context.Context, count int64, block time.Duration) ([]Delivery, error) {
	res, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumerName,
		Streams:  []string{q.stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var out []Delivery
	for _, stream := range res {
		for _, msg := range stream.Messages {
			job, ok := decodeJob(msg.Values)
			if !ok {
				// Poison message: can't be decoded back into a job. Ack it
				// so it doesn't block the group forever; it's logged for
				// manual inspection since we have no job ID to report.
				log.Printf("queue: dropping undecodable message %s from %s", msg.ID, q.stream)
				_ = q.Ack(ctx, msg.ID)
				continue
			}
			out = append(out, Delivery{Job: job, ID: msg.ID})
		}
	}
	return out, nil
}

func (q *RedisStream) Ack(ctx context.Context, id string) error {
	return q.rdb.XAck(ctx, q.stream, q.group, id).Err()
}

func (q *RedisStream) Nack(ctx context.Context, id string, reason error) error {
	if reason != nil {
		// Best-effort: remembered so a later dead-letter has a real
		// failure reason to report, not just "max deliveries exceeded".
		q.rdb.Set(ctx, q.lastErrorKey(id), reason.Error(), dedupeTTL)
	}
	return nil
}

func (q *RedisStream) lastErrorKey(id string) string {
	return fmt.Sprintf("streamroom:joberr:%s:%s", q.stream, id)
}

func (q *RedisStream) Reclaim(ctx context.Context, minIdle time.Duration, maxDeliveries int64) ([]Delivery, []Delivery, error) {
	pending, err := q.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: q.stream,
		Group:  q.group,
		Start:  "-",
		End:    "+",
		Count:  100,
		Idle:   minIdle,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("xpending: %w", err)
	}
	if len(pending) == 0 {
		return nil, nil, nil
	}

	var toClaim []string
	var toDeadLetter []string
	for _, p := range pending {
		if p.RetryCount >= maxDeliveries {
			toDeadLetter = append(toDeadLetter, p.ID)
			continue
		}
		toClaim = append(toClaim, p.ID)
	}

	var deadLettered []Delivery
	for _, id := range toDeadLetter {
		job, reason, err := q.deadLetterOne(ctx, id)
		if err != nil {
			log.Printf("queue: failed to dead-letter %s: %v", id, err)
			continue
		}
		if job != nil {
			deadLettered = append(deadLettered, Delivery{Job: *job, ID: id, LastError: reason})
		}
	}

	if len(toClaim) == 0 {
		return nil, deadLettered, nil
	}

	msgs, err := q.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   q.stream,
		Group:    q.group,
		Consumer: q.consumerName,
		MinIdle:  minIdle,
		Messages: toClaim,
	}).Result()
	if err != nil {
		return nil, deadLettered, fmt.Errorf("xclaim: %w", err)
	}

	var reclaimed []Delivery
	for _, msg := range msgs {
		job, ok := decodeJob(msg.Values)
		if !ok {
			log.Printf("queue: dropping undecodable reclaimed message %s from %s", msg.ID, q.stream)
			_ = q.Ack(ctx, msg.ID)
			continue
		}
		job.Attempt++
		reclaimed = append(reclaimed, Delivery{Job: job, ID: msg.ID})
	}
	return reclaimed, deadLettered, nil
}

// deadLetterOne moves a single pending entry to the dead-letter stream and
// acks it off the main stream so it stops showing up in future XPENDING
// scans. Returns the decoded job (for the caller to report on) and the last
// recorded failure reason, if any.
func (q *RedisStream) deadLetterOne(ctx context.Context, id string) (*TranscodeJob, string, error) {
	msgs, err := q.rdb.XRange(ctx, q.stream, id, id).Result()
	if err != nil {
		return nil, "", fmt.Errorf("xrange: %w", err)
	}

	var job *TranscodeJob
	var reason string
	if len(msgs) == 1 {
		vals := msgs[0].Values
		if decoded, ok := decodeJob(vals); ok {
			job = &decoded
		}
		reason, _ = q.rdb.Get(ctx, q.lastErrorKey(id)).Result()

		dlqVals := map[string]interface{}{
			"job":        vals["job"],
			"failed_id":  id,
			"last_error": reason,
		}
		if err := q.rdb.XAdd(ctx, &redis.XAddArgs{Stream: q.deadLetter, Values: dlqVals}).Err(); err != nil {
			return job, reason, fmt.Errorf("xadd dlq: %w", err)
		}
	}

	if err := q.rdb.XAck(ctx, q.stream, q.group, id).Err(); err != nil {
		return job, reason, fmt.Errorf("xack: %w", err)
	}
	q.rdb.Del(ctx, q.lastErrorKey(id))
	return job, reason, nil
}

func decodeJob(values map[string]interface{}) (TranscodeJob, bool) {
	raw, ok := values["job"]
	if !ok {
		return TranscodeJob{}, false
	}
	s, ok := raw.(string)
	if !ok {
		return TranscodeJob{}, false
	}
	var job TranscodeJob
	if err := json.Unmarshal([]byte(s), &job); err != nil {
		return TranscodeJob{}, false
	}
	return job, true
}
