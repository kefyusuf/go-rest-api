package ratelimit_test

import (
	"testing"
	"time"

	"go-lang/internal/ratelimit"
)

func TestLimiterAllowsUpToBurst(t *testing.T) {
	l := ratelimit.New(0, 3)
	for i := 0; i < 3; i++ {
		if !l.Allow("client-1") {
			t.Fatalf("expected allow on request %d", i+1)
		}
	}
	if l.Allow("client-1") {
		t.Fatal("expected 4th request to be denied")
	}
}

func TestLimiterIsPerKey(t *testing.T) {
	l := ratelimit.New(0, 1)

	if !l.Allow("client-1") {
		t.Fatal("expected first request from client-1 to be allowed")
	}
	if l.Allow("client-1") {
		t.Fatal("expected second request from client-1 to be denied")
	}
	if !l.Allow("client-2") {
		t.Fatal("expected first request from client-2 to be allowed")
	}
}

func TestLimiterRefillsOverTime(t *testing.T) {
	now := time.Now()
	current := now
	l := ratelimit.NewWithNow(2, 1, func() time.Time { return current })

	if !l.Allow("client-1") {
		t.Fatal("expected first request to be allowed")
	}
	if l.Allow("client-1") {
		t.Fatal("expected second request to be denied")
	}

	current = current.Add(time.Second)

	if !l.Allow("client-1") {
		t.Fatal("expected request after refill to be allowed")
	}
}

func TestLimiterRetryAfterIsZeroWhenBucketFull(t *testing.T) {
	l := ratelimit.New(1, 2)
	_ = l.Allow("client-1")
	if got := l.RetryAfter("client-1"); got != 0 {
		t.Fatalf("expected 0 when bucket has tokens, got %s", got)
	}
}

func TestLimiterRetryAfterWhenDenied(t *testing.T) {
	l := ratelimit.New(1, 1)
	_ = l.Allow("client-1")
	got := l.RetryAfter("client-1")
	if got <= 0 {
		t.Fatalf("expected positive retry, got %s", got)
	}
}
