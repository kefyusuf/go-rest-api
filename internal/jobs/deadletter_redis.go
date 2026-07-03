package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisDeadLetter is the Redis-backed implementation of
// DeadLetter. Each Add appends a JSON-serialised DeadLetterEntry
// to a single Redis list keyed by the configured stream name.
// List returns the entries in insertion order (oldest first).
// The list is unbounded; in a long-running production
// deployment a cleanup job should trim it. A bounded
// implementation (e.g. with LTRIM after each Add) is a one-line
// change.
type redisDeadLetter struct {
	client *redis.Client
	key    string
}

// NewRedisDeadLetter builds a Redis-backed DeadLetter on top of
// an existing client. The list does not own the client; the
// caller is responsible for closing it.
func NewRedisDeadLetter(client *redis.Client) *redisDeadLetter {
	return &redisDeadLetter{client: client, key: "go-rest-api.jobs.dlq"}
}

func (d *redisDeadLetter) Add(job Job, lastErr string) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	entry := DeadLetterEntry{
		Job:     job,
		LastErr: lastErr,
		At:      job.EnqueuedAt,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// RPUSH appends to the tail. We do not LPUSH to a bounded
	// length here; the caller is expected to trim the list
	// periodically. Errors are best-effort: a transient
	// Redis failure means the next dispatcher's list view
	// will be missing this entry, but the underlying job is
	// already past the retry budget so the user-visible
	// behaviour is the same.
	_ = d.client.RPush(ctx, d.key, data).Err()
}

func (d *redisDeadLetter) List() []DeadLetterEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	vals, err := d.client.LRange(ctx, d.key, 0, -1).Result()
	if err != nil {
		return nil
	}
	out := make([]DeadLetterEntry, 0, len(vals))
	for _, v := range vals {
		var e DeadLetterEntry
		if err := json.Unmarshal([]byte(v), &e); err != nil {
			// Skip corrupt entries rather than failing the
			// whole List call. A clean-up job can identify
			// and remove the corrupt row.
			continue
		}
		out = append(out, e)
	}
	return out
}

// Compile-time assertion: the Redis implementation satisfies
// DeadLetter.
var _ DeadLetter = (*redisDeadLetter)(nil)
