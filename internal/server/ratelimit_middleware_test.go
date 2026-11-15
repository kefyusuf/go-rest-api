package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-lang/internal/server"
)

// The Redis limiter derives its bucket keys from the name the
// middleware passes, so the auth limiter must namespace its keys away
// from the global limiter's: sharing a key would make both limiters
// drain the same "ratelimit:<ip>:bucket".
func TestAuthAndGlobalLimiterKeysDiffer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:12345"

	global := server.ClientIPKey(r)
	auth := server.AuthClientIPKey(r)

	if auth == global {
		t.Fatalf("auth key %q must differ from global key %q", auth, global)
	}
	if auth != "auth:"+global {
		t.Fatalf("expected auth key %q to be the namespaced global key %q", auth, global)
	}
}
