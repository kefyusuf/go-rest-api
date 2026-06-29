package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-lang/internal/auth"
	"go-lang/internal/ratelimit"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

func TestSecurityHeadersOnAllResponses(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{BcryptCost: 4})
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options=nosniff, got %q", got)
	}
	if got := res.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options=DENY, got %q", got)
	}
	if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("expected Referrer-Policy=no-referrer, got %q", got)
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{
		BcryptCost: 4,
		CORS: server.CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
		},
	})
	ts := httptest.NewServer(app)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	req.Header.Set("Origin", "https://example.com")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("expected Access-Control-Allow-Origin, got %q", got)
	}
	if got := res.Header.Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary=Origin, got %q", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{
		BcryptCost: 4,
		CORS: server.CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
		},
	})
	ts := httptest.NewServer(app)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header, got %q", got)
	}
}

func TestCORSPreflightReturns204(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{
		BcryptCost: 4,
		CORS: server.CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
		},
	})
	ts := httptest.NewServer(app)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/users", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.StatusCode)
	}
	if !strings.Contains(res.Header.Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("expected Access-Control-Allow-Methods to include POST, got %q", res.Header.Get("Access-Control-Allow-Methods"))
	}
}

func TestRateLimitReturns429(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{
		BcryptCost:     4,
		GlobalLimiter:  limiter,
		AuthLimiter:    limiter,
	})
	ts := httptest.NewServer(app)
	defer ts.Close()

	for i := 0; i < 5; i++ {
		res, err := http.Get(ts.URL + "/users")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			body := res.Body
			var payload map[string]any
			if err := json.NewDecoder(body).Decode(&payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := res.Header.Get("Retry-After"); got == "" {
				t.Fatal("expected Retry-After header on rate-limit response")
			}
			errMap, ok := payload["error"].(map[string]any)
			if !ok {
				t.Fatalf("expected error envelope, got %v", payload)
			}
			if errMap["code"] != "RATE_LIMITED" {
				t.Fatalf("expected RATE_LIMITED, got %v", errMap["code"])
			}
			res.Body.Close()
			return
		}
		res.Body.Close()
	}
	t.Fatal("expected at least one 429 after 5 requests with a 1 rps / 1 burst limiter")
}

func TestAuthRateLimitIndependentBucket(t *testing.T) {
	globalLimiter := ratelimit.New(100, 200)
	authLimiter := ratelimit.New(1, 1)

	access, _ := auth.NewTokenIssuer(testJWTSecret, 15*time.Minute, "test", auth.KindAccess)
	refresh, _ := auth.NewTokenIssuer(testJWTSecret, 24*time.Hour, "test", auth.KindRefresh)
	blacklist := auth.NewBlacklist()

	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{
		BcryptCost:     4,
		GlobalLimiter:  globalLimiter,
		AuthLimiter:    authLimiter,
		TokenIssuer:    access,
		RefreshIssuer:  refresh,
		Blacklist:      blacklist,
	})
	ts := httptest.NewServer(app)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"email": "x@x.com", "password": "x"})
	var last int
	for i := 0; i < 5; i++ {
		res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		last = res.StatusCode
		res.Body.Close()
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("expected 429 from auth limiter, got %d", last)
	}

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected /health to stay unaffected by auth limiter, got %d", res.StatusCode)
	}
}
