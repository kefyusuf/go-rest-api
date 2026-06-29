package jobs

import (
	"sync"
)

type memoryDeadLetter struct {
	mu      sync.Mutex
	entries []DeadLetterEntry
}

func NewMemoryDeadLetter() *memoryDeadLetter {
	return &memoryDeadLetter{}
}

func (d *memoryDeadLetter) Add(job Job, lastErr string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = append(d.entries, DeadLetterEntry{
		Job:     job,
		LastErr: lastErr,
		At:      job.EnqueuedAt,
	})
}

func (d *memoryDeadLetter) List() []DeadLetterEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]DeadLetterEntry, len(d.entries))
	copy(out, d.entries)
	return out
}
