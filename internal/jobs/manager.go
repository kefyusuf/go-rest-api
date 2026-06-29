package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Status struct {
	Pending  int            `json:"pending"`
	Handlers map[string]int `json:"handlers"`
	Started  bool           `json:"started"`
	Workers  int            `json:"workers"`
}

type Manager struct {
	mu       sync.RWMutex
	registry *Registry
	queue    Queue
	dead     DeadLetter
	workers  int
}

func NewManager(registry *Registry, queue Queue, dead DeadLetter) *Manager {
	return &Manager{
		registry: registry,
		queue:    queue,
		dead:     dead,
	}
}

func (m *Manager) Enqueue(ctx Context, jobType string, payload []byte) error {
	return m.registry.Enqueue(ctx, jobType, payload)
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s := Status{
		Pending:  m.queue.Len(),
		Handlers: make(map[string]int),
		Started:  m.registry.started,
		Workers:  m.workers,
	}
	if m.dead != nil {
		entries := m.dead.List()
		for _, e := range entries {
			s.Handlers[e.Job.Type]++
		}
	}
	return s
}

func (m *Manager) DeadLetters() []DeadLetterEntry {
	if m.dead == nil {
		return nil
	}
	entries := m.dead.List()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].At.After(entries[j].At)
	})
	return entries
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("job-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

type Context = context.Context
