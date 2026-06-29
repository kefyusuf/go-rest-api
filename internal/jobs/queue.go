// Package jobs implements a small background-job queue with retry,
// exponential backoff, and a dead-letter list. The in-memory
// implementation is a channel-based worker pool suitable for the
// starter; production deployments would swap the queue for RabbitMQ
// or another broker while keeping the same Handler interface.
package jobs

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrQueueClosed = errors.New("queue is closed")

type Job struct {
	ID         string
	Type       string
	Payload    []byte
	Attempts   int
	MaxRetries int
	EnqueuedAt time.Time
	RunAfter   time.Time
	LastError  string
}

type Handler interface {
	Handle(ctx context.Context, job Job) error
}

type HandlerFunc func(ctx context.Context, job Job) error

func (f HandlerFunc) Handle(ctx context.Context, job Job) error {
	return f(ctx, job)
}

type Queue interface {
	Enqueue(ctx context.Context, job Job) error
	Dequeue(ctx context.Context) (Job, bool, error)
	Ack(ctx context.Context, job Job) error
	Nack(ctx context.Context, job Job, err error) error
	Len() int
	Close() error
}

type DeadLetter interface {
	Add(job Job, lastErr string)
	List() []DeadLetterEntry
}

type DeadLetterEntry struct {
	Job     Job
	LastErr string
	At      time.Time
}

type memoryQueue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	items    []Job
	closed   bool
}

func NewMemoryQueue() *memoryQueue {
	q := &memoryQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *memoryQueue) Enqueue(ctx context.Context, job Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrQueueClosed
	}
	q.items = append(q.items, job)
	q.cond.Signal()
	return nil
}

func (q *memoryQueue) Dequeue(ctx context.Context) (Job, bool, error) {
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return Job{}, false, nil
		}
		if len(q.items) == 0 {
			q.cond.Wait()
			q.mu.Unlock()
			continue
		}
		job := q.items[0]
		q.items = q.items[1:]
		if job.RunAfter.After(time.Now()) {
			q.items = append(q.items, job)
			q.mu.Unlock()
			select {
			case <-ctx.Done():
				return Job{}, false, nil
			case <-time.After(time.Until(job.RunAfter)):
				continue
			}
		}
		q.mu.Unlock()
		return job, true, nil
	}
}

func (q *memoryQueue) Ack(ctx context.Context, job Job) error {
	return nil
}

func (q *memoryQueue) Nack(ctx context.Context, job Job, err error) error {
	job.LastError = errString(err)
	if job.Attempts > job.MaxRetries {
		return nil
	}
	job.RunAfter = time.Now().Add(backoff(job.Attempts))
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrQueueClosed
	}
	q.items = append(q.items, job)
	q.cond.Signal()
	return nil
}

func (q *memoryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *memoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	q.cond.Broadcast()
	return nil
}

func backoff(attempt int) time.Duration {
	base := time.Second
	for i := 1; i < attempt; i++ {
		base *= 2
		if base > time.Minute {
			return time.Minute
		}
	}
	return base
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
