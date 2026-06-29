package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Registry struct {
	mu        sync.RWMutex
	handlers  map[string]Handler
	queue     Queue
	dead      DeadLetter
	logger    *slog.Logger
	started   bool
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

func NewRegistry(queue Queue, dead DeadLetter, logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		handlers: make(map[string]Handler),
		queue:    queue,
		dead:     dead,
		logger:   logger,
	}
}

func (r *Registry) Register(jobType string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[jobType] = h
}

func (r *Registry) Enqueue(ctx context.Context, jobType string, payload []byte) error {
	job := Job{
		ID:         newID(),
		Type:       jobType,
		Payload:    payload,
		MaxRetries: 2,
		EnqueuedAt: time.Now(),
		RunAfter:   time.Now(),
	}
	return r.queue.Enqueue(ctx, job)
}

func (r *Registry) Start(ctx context.Context, workers int) {
	if workers <= 0 {
		workers = 1
	}
	r.startOnce.Do(func() {
		r.started = true
		for i := 0; i < workers; i++ {
			r.wg.Add(1)
			go r.workerLoop(ctx, i)
		}
	})
}

func (r *Registry) Stop() {
	r.stopOnce.Do(func() {
		_ = r.queue.Close()
		r.wg.Wait()
	})
}

func (r *Registry) workerLoop(ctx context.Context, id int) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, ok, err := r.queue.Dequeue(ctx)
		if err != nil {
			r.logger.Error("queue dequeue failed",
				slog.Int("worker", id),
				slog.String("error", err.Error()))
			return
		}
		if !ok {
			return
		}

		r.runJob(ctx, id, job)
	}
}

func (r *Registry) runJob(ctx context.Context, workerID int, job Job) {
	r.mu.RLock()
	h, ok := r.handlers[job.Type]
	r.mu.RUnlock()

	if !ok {
		r.logger.Warn("job has no registered handler",
			slog.Int("worker", workerID),
			slog.String("job_id", job.ID),
			slog.String("job_type", job.Type))
		job.Attempts++
		err := errNoHandler{jobType: job.Type}
		if job.Attempts > job.MaxRetries {
			if r.dead != nil {
				r.dead.Add(job, err.Error())
			}
			return
		}
		_ = r.queue.Nack(ctx, job, err)
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	job.Attempts++
	err := h.Handle(jobCtx, job)
	if err == nil {
		_ = r.queue.Ack(jobCtx, job)
		return
	}

	r.logger.Warn("job failed",
		slog.Int("worker", workerID),
		slog.String("job_id", job.ID),
		slog.String("job_type", job.Type),
		slog.Int("attempts", job.Attempts),
		slog.String("error", err.Error()))

	if job.Attempts > job.MaxRetries {
		if r.dead != nil {
			r.dead.Add(job, err.Error())
		}
		r.logger.Error("job moved to dead-letter",
			slog.String("job_id", job.ID),
			slog.String("job_type", job.Type),
			slog.Int("attempts", job.Attempts))
		return
	}

	if nerr := r.queue.Nack(jobCtx, job, err); nerr != nil {
		r.logger.Error("queue nack failed",
			slog.String("error", nerr.Error()))
	}
}

type errNoHandler struct{ jobType string }

func (e errNoHandler) Error() string { return "no handler for job type " + e.jobType }
