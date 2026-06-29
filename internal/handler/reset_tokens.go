package handler

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type resetToken struct {
	userID    int64
	expiresAt time.Time
}

type resetTokenStore struct {
	mu      sync.Mutex
	tokens  map[string]resetToken
}

func newResetTokenStore() *resetTokenStore {
	return &resetTokenStore{tokens: make(map[string]resetToken)}
}

func (s *resetTokenStore) Issue(userID int64, now func() time.Time, ttl time.Duration) (string, time.Time) {
	token := randomToken(32)
	expiresAt := now().Add(ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = resetToken{userID: userID, expiresAt: expiresAt}
	return token, expiresAt
}

func (s *resetTokenStore) Consume(token string, now func() time.Time) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tokens[token]
	if !ok {
		return 0, false
	}
	delete(s.tokens, token)
	if now().After(entry.expiresAt) {
		return 0, false
	}
	return entry.userID, true
}

func randomToken(byteLen int) string {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
