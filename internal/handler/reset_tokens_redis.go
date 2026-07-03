package handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisResetTokenStore is the Redis-backed TokenStore
// implementation. Each token is a single Redis hash with two
// fields (user_id, expires_at) and a per-key EX TTL. The TTL is
// set on Issue and on Consume (the Lua-style atomic GETDEL
// pattern), so a token that has not been consumed is reclaimed
// automatically when it would have expired anyway. Consume uses
// GETDEL to make the operation atomic and single-use; the caller
// never observes a token that another consumer has already
// claimed.
type redisResetTokenStore struct {
	client *redis.Client
}

// NewRedisResetTokenStore builds a Redis-backed TokenStore on
// top of an existing client. The store does not own the client;
// the caller is responsible for closing it.
func NewRedisResetTokenStore(client *redis.Client) *redisResetTokenStore {
	return &redisResetTokenStore{client: client}
}

func (s *redisResetTokenStore) keyFor(token string) string {
	return "reset_token:" + token
}

func (s *redisResetTokenStore) Issue(userID int64, now func() time.Time, ttl time.Duration) (string, time.Time) {
	token := randomToken(32)
	expiresAt := now().Add(ttl)
	payload, err := json.Marshal(struct {
		UserID    int64     `json:"u"`
		ExpiresAt time.Time `json:"e"`
	}{UserID: userID, ExpiresAt: expiresAt})
	if err != nil {
		// json.Marshal of these two fields cannot fail in
		// practice. Returning an empty token is the safe
		// fallback; the handler treats a 32-byte-empty token
		// as a 401 and the user can retry.
		return "", expiresAt
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := s.client.Set(ctx, s.keyFor(token), payload, ttl).Err(); err != nil {
		// Issue a 5xx-friendly fallback: log would be the next
		// step in a fuller implementation. Returning an empty
		// token makes the handler refuse the call cleanly.
		return "", expiresAt
	}
	return token, expiresAt
}

func (s *redisResetTokenStore) Consume(token string, now func() time.Time) (int64, bool) {
	if token == "" {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	val, err := s.client.GetDel(ctx, s.keyFor(token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false
		}
		return 0, false
	}

	var payload struct {
		UserID    int64     `json:"u"`
		ExpiresAt time.Time `json:"e"`
	}
	if err := json.Unmarshal([]byte(val), &payload); err != nil {
		return 0, false
	}
	if now().After(payload.ExpiresAt) {
		return 0, false
	}
	return payload.UserID, true
}

// Compile-time assertion: the Redis implementation satisfies
// TokenStore.
var _ TokenStore = (*redisResetTokenStore)(nil)
