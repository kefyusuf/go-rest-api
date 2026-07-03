package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"go-lang/internal/auth"
)

// redisClient returns a Redis client connected to localhost. The test
// is skipped when Redis is not reachable, so the suite stays green
// on machines without a Redis daemon.
func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:6379",
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not reachable, skipping: %v", err)
	}
	return c
}

func TestRedisBlacklistRevokeAndCheck(t *testing.T) {
	client := redisClient(t)
	defer client.Close()

	jti := "test-revoke-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), "auth:blacklist:"+jti)

	bl := auth.NewRedisBlacklist(client)
	if bl.IsRevoked(jti) {
		t.Fatal("expected fresh jti to not be revoked")
	}

	expires := time.Now().Add(time.Hour)
	bl.Revoke(jti, expires)
	if !bl.IsRevoked(jti) {
		t.Fatal("expected revoked jti to be detected")
	}
}

func TestRedisBlacklistExpiredJtiIsNotRevoked(t *testing.T) {
	client := redisClient(t)
	defer client.Close()

	jti := "test-expired-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), "auth:blacklist:"+jti)

	bl := auth.NewRedisBlacklist(client)
	// Revoke with an already-expired timestamp: the implementation
	// should treat the call as a no-op because the underlying SET
	// with a non-positive TTL would be rejected by Redis.
	bl.Revoke(jti, time.Now().Add(-time.Minute))
	if bl.IsRevoked(jti) {
		t.Fatal("expected already-expired jti to not be revoked")
	}
}
