package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type blacklistEntry struct {
	expiresAt time.Time
}

type Blacklist struct {
	mu      sync.Mutex
	entries map[string]blacklistEntry
}

func NewBlacklist() *Blacklist {
	return &Blacklist{entries: make(map[string]blacklistEntry)}
}

func (b *Blacklist) Revoke(jti string, expiresAt time.Time) {
	if jti == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = blacklistEntry{expiresAt: expiresAt}
}

func (b *Blacklist) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.entries[jti]
	if !ok {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		delete(b.entries, jti)
		return false
	}
	return true
}

func newJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
