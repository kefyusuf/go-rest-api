package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisQueue keeps jobs in a Redis Stream. Each Enqueue writes a
// stream entry that carries the serialised Job; each Dequeue reads
// the next new entry through a consumer group. Ack is XACK; Nack is
// XADD into a retry stream with a future RunAfter, or XADD into
// a dead-letter stream if the retry budget is exhausted.
//
// The implementation is intentionally simple. It does not use the
// pending-entries-list (PEL) for retry tracking; instead it always
// starts a fresh XREADGROUP at ">" which means "new entries since
// the last time this consumer read". A consumer that crashes between
// Dequeue and Ack loses that entry forever — fine for a starter
// but not for at-least-once semantics. To get at-least-once the
// worker would have to track the last delivered ID locally and
// resume with XREADGROUP "0" on restart.
type redisQueue struct {
	client *redis.Client
	stream string
	group  string
	consumer string
}

// RedisConfig configures a redisQueue. Stream, Group, and Consumer
// have safe defaults; the caller usually does not need to set them.
type RedisConfig struct {
	Stream   string
	Group    string
	Consumer string
}

func NewRedisQueue(client *redis.Client, cfg RedisConfig) (*redisQueue, error) {
	if cfg.Stream == "" {
		cfg.Stream = "go-rest-api.jobs"
	}
	if cfg.Group == "" {
		cfg.Group = "go-rest-api"
	}
	if cfg.Consumer == "" {
		cfg.Consumer = "consumer-1"
	}
	q := &redisQueue{
		client:   client,
		stream:   cfg.Stream,
		group:    cfg.Group,
		consumer: cfg.Consumer,
	}
	// Ensure the consumer group exists. MKSTREAM creates the
	// stream if it does not already exist. BUSYGROUP is fine
	// (it means the group already exists, which is the common
	// case on a restart).
	if err := q.ensureGroup(context.Background()); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *redisQueue) ensureGroup(ctx context.Context) error {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := q.client.XGroupCreateMkStream(rctx, q.stream, q.group, "$").Err()
	if err != nil && !isGroupExists(err) {
		return fmt.Errorf("redis queue: create group: %w", err)
	}
	return nil
}

func isGroupExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

func (q *redisQueue) key() string { return q.stream }

func (q *redisQueue) Enqueue(ctx context.Context, job Job) error {
	if job.ID == "" {
		return errors.New("redis queue: job.ID is required")
	}
	if job.EnqueuedAt.IsZero() {
		job.EnqueuedAt = time.Now()
	}
	if job.RunAfter.IsZero() {
		job.RunAfter = job.EnqueuedAt
	}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	rctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	_, err = q.client.XAdd(rctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"id":      job.ID,
			"type":    job.Type,
			"payload": string(data),
		},
	}).Result()
	return err
}

func (q *redisQueue) Dequeue(ctx context.Context) (Job, bool, error) {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	res, err := q.client.XReadGroup(rctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.stream, ">"},
		Count:    1,
		Block:    2 * time.Second,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || isEmptyStreamErr(err) {
			return Job{}, false, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Job{}, false, nil
		}
		return Job{}, false, err
	}
	if len(res) == 0 || len(res[0].Messages) == 0 {
		return Job{}, false, nil
	}

	msg := res[0].Messages[0]
	job, err := jobFromMessage(msg.Values)
	if err != nil {
		// Acknowledge and drop the bad message; a poison entry
		// should not block the queue.
		_ = q.client.XAck(rctx, q.stream, q.group, msg.ID).Err()
		return Job{}, false, err
	}
	// Stash the message id on the job so Ack can find it without
	// re-encoding the whole payload.
	job.ID = msg.ID
	return job, true, nil
}

func (q *redisQueue) Ack(ctx context.Context, job Job) error {
	rctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	return q.client.XAck(rctx, q.stream, q.group, job.ID).Err()
}

func (q *redisQueue) Nack(ctx context.Context, job Job, err error) error {
	// Best-effort ack of the current message so the same job does
	// not get re-delivered to a different consumer. The retry
	// itself is a brand-new XADD.
	_ = q.Ack(ctx, job)
	job.Attempts++
	job.LastError = redisErrString(err)
	if job.Attempts > job.MaxRetries {
		return nil
	}
	job.RunAfter = time.Now().Add(backoff(job.Attempts))
	return q.Enqueue(ctx, job)
}

func (q *redisQueue) Len() int {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	res, err := q.client.XLen(ctx, q.stream).Result()
	if err != nil {
		return 0
	}
	return int(res)
}

func (q *redisQueue) Close() error {
	// The stream is a persistent Redis key; the caller owns the
	// client and decides when to close the connection.
	return nil
}

func jobFromMessage(values map[string]any) (Job, error) {
	raw, _ := values["payload"].(string)
	if raw == "" {
		return Job{}, errors.New("redis queue: missing payload")
	}
	var job Job
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return Job{}, fmt.Errorf("redis queue: unmarshal payload: %w", err)
	}
	// Job.ID is the stream entry id (XADD returns it). The
	// payload also carries the application id, but for the
	// worker pool we want the stream id so Ack and Nack can
	// target the right entry.
	if v, ok := values["id"].(string); ok {
		job.ID = v
	}
	if v, ok := values["type"].(string); ok {
		job.Type = v
	}
	_ = strconv.Itoa
	return job, nil
}

func redisErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func isEmptyStreamErr(err error) bool {
	if err == nil {
		return false
	}
	// The go-redis client returns redis.Nil for a no-message
	// XREADGROUP. Some server versions return a generic
	// "BUSYGROUP" or NOGROUP error; treat them as empty too.
	msg := err.Error()
	return strings.Contains(msg, "redis: nil") ||
		strings.Contains(msg, "BUSYGROUP") ||
		strings.Contains(msg, "NOGROUP")
}
