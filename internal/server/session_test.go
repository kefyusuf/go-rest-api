package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-lang/internal/auth"
	"go-lang/internal/model"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

const (
	sessionJWTSecret = "this-is-a-test-jwt-secret-of-at-least-32-bytes"
)

func newSessionApp(t *testing.T) (*httptest.Server, *auth.TokenIssuer, *auth.TokenIssuer, *auth.Blacklist, store.UserStore) {
	t.Helper()
	userStore := store.NewMemoryUserStore()
	access, err := auth.NewTokenIssuer(sessionJWTSecret, 15*time.Minute, "test", auth.KindAccess)
	if err != nil {
		t.Fatalf("access issuer: %v", err)
	}
	refresh, err := auth.NewTokenIssuer(sessionJWTSecret, 24*time.Hour, "test", auth.KindRefresh)
	if err != nil {
		t.Fatalf("refresh issuer: %v", err)
	}
	blacklist := auth.NewBlacklist()
	app := server.New(userStore, newTestLogger(), server.Options{
		BcryptCost:    4,
		TokenIssuer:   access,
		RefreshIssuer: refresh,
		Blacklist:     blacklist,
	})
	ts := httptest.NewServer(app)
	return ts, access, refresh, blacklist, userStore
}

func registerUser(t *testing.T, ts *httptest.Server, name, email, password string) model.LoginResponse {
	t.Helper()
	body, _ := json.Marshal(model.CreateUserRequest{
		Name: name, Email: email, Password: password,
	})
	res, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	var out model.LoginResponse
	decodeJSON(t, res.Body, &out)
	return out
}

func TestRegisterCreatesUserAndReturnsTokens(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	out := registerUser(t, ts, "Ada", "ada@example.com", "pw")
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens on register")
	}
	if out.User.Email != "ada@example.com" {
		t.Fatalf("expected ada@example.com, got %q", out.User.Email)
	}
	if out.User.PasswordHash != "" {
		t.Fatalf("password hash leaked on register")
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	registerUser(t, ts, "Ada", "ada@example.com", "pw")

	body, _ := json.Marshal(model.CreateUserRequest{
		Name: "Other", Email: "ada@example.com", Password: "pw2",
	})
	res, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", res.StatusCode)
	}
}

func TestRegisterValidationRejectsMissingFields(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	body, _ := json.Marshal(model.CreateUserRequest{Name: "", Email: "", Password: ""})
	res, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	var payload model.ErrorResponse
	decodeJSON(t, res.Body, &payload)
	if payload.Error.Details == nil {
		t.Fatal("expected validation details")
	}
}

func TestRegisterWrongContentType(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/auth/register", "text/plain", bytes.NewReader([]byte("ok")))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", res.StatusCode)
	}
}

func TestLoginReturnsRefreshToken(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	registerUser(t, ts, "Ada", "ada@example.com", "pw")

	body, _ := json.Marshal(model.LoginRequest{Email: "ada@example.com", Password: "pw"})
	res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var out model.LoginResponse
	decodeJSON(t, res.Body, &out)
	if out.RefreshToken == "" {
		t.Fatal("expected refresh token on login")
	}
}

func TestLogoutRevokesAccessToken(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	session := registerUser(t, ts, "Ada", "ada@example.com", "pw")

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.StatusCode)
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+session.AccessToken)
	meRes, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer meRes.Body.Close()
	if meRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked token to fail /me with 401, got %d", meRes.StatusCode)
	}
}

func TestLogoutRejectsMissingHeader(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func TestRefreshIssuesNewTokens(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	session := registerUser(t, ts, "Ada", "ada@example.com", "pw")

	body, _ := json.Marshal(model.RefreshRequest{RefreshToken: session.RefreshToken})
	res, err := http.Post(ts.URL+"/auth/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var out model.LoginResponse
	decodeJSON(t, res.Body, &out)
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatal("expected new tokens on refresh")
	}
	if out.RefreshToken == session.RefreshToken {
		t.Fatal("expected refresh token rotation, got the same token back")
	}
}

func TestRefreshRejectsAlreadyUsedToken(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	session := registerUser(t, ts, "Ada", "ada@example.com", "pw")

	body, _ := json.Marshal(model.RefreshRequest{RefreshToken: session.RefreshToken})
	first, err := http.Post(ts.URL+"/auth/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("expected first refresh 200, got %d", first.StatusCode)
	}

	replay, err := http.Post(ts.URL+"/auth/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer replay.Body.Close()
	if replay.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected replay to be 401, got %d", replay.StatusCode)
	}
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	session := registerUser(t, ts, "Ada", "ada@example.com", "pw")

	body, _ := json.Marshal(model.RefreshRequest{RefreshToken: session.AccessToken})
	res, err := http.Post(ts.URL+"/auth/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when sending an access token, got %d", res.StatusCode)
	}
}

func TestForgotPasswordReturns202(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	registerUser(t, ts, "Ada", "ada@example.com", "pw")

	body, _ := json.Marshal(model.ForgotPasswordRequest{Email: "ada@example.com"})
	res, err := http.Post(ts.URL+"/auth/forgot-password", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("forgot: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", res.StatusCode)
	}

	var out model.ForgotPasswordResponse
	decodeJSON(t, res.Body, &out)
	if !out.Accepted {
		t.Fatal("expected accepted=true")
	}
	if out.Token == "" {
		t.Fatal("expected a reset token in the response (in-memory starter)")
	}
}

func TestForgotPasswordAlwaysReturns202ForUnknownEmail(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	body, _ := json.Marshal(model.ForgotPasswordRequest{Email: "ghost@example.com"})
	res, err := http.Post(ts.URL+"/auth/forgot-password", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("forgot: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 even for unknown email, got %d", res.StatusCode)
	}

	var out model.ForgotPasswordResponse
	decodeJSON(t, res.Body, &out)
	if out.Token != "" {
		t.Fatal("expected no token in the response for unknown email")
	}
}

func TestResetPasswordChangesPassword(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	registerUser(t, ts, "Ada", "ada@example.com", "old-password")

	body, _ := json.Marshal(model.ForgotPasswordRequest{Email: "ada@example.com"})
	forgotRes, err := http.Post(ts.URL+"/auth/forgot-password", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("forgot: %v", err)
	}
	defer forgotRes.Body.Close()
	var forgotOut model.ForgotPasswordResponse
	decodeJSON(t, forgotRes.Body, &forgotOut)

	resetBody, _ := json.Marshal(model.ResetPasswordRequest{
		Token:    forgotOut.Token,
		Password: "new-password",
	})
	resetRes, err := http.Post(ts.URL+"/auth/reset-password", "application/json", bytes.NewReader(resetBody))
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	defer resetRes.Body.Close()
	if resetRes.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resetRes.StatusCode)
	}

	loginOld, _ := json.Marshal(model.LoginRequest{Email: "ada@example.com", Password: "old-password"})
	oldRes, _ := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(loginOld))
	oldRes.Body.Close()
	if oldRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password should fail after reset, got %d", oldRes.StatusCode)
	}

	loginNew, _ := json.Marshal(model.LoginRequest{Email: "ada@example.com", Password: "new-password"})
	newRes, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(loginNew))
	if err != nil {
		t.Fatalf("login new: %v", err)
	}
	defer newRes.Body.Close()
	if newRes.StatusCode != http.StatusOK {
		t.Fatalf("new password should succeed, got %d", newRes.StatusCode)
	}
}

func TestResetPasswordRejectsReusedToken(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()
	registerUser(t, ts, "Ada", "ada@example.com", "old-password")

	body, _ := json.Marshal(model.ForgotPasswordRequest{Email: "ada@example.com"})
	forgotRes, err := http.Post(ts.URL+"/auth/forgot-password", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatalf("forgot: %v", err) }
	defer forgotRes.Body.Close()
	var forgotOut model.ForgotPasswordResponse
	decodeJSON(t, forgotRes.Body, &forgotOut)

	resetBody, _ := json.Marshal(model.ResetPasswordRequest{Token: forgotOut.Token, Password: "new-pw"})
	first, err := http.Post(ts.URL+"/auth/reset-password", "application/json", bytes.NewReader(resetBody))
	if err != nil { t.Fatalf("first reset: %v", err) }
	first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("expected first reset 204, got %d", first.StatusCode)
	}

	second, err := http.Post(ts.URL+"/auth/reset-password", "application/json", bytes.NewReader(resetBody))
	if err != nil { t.Fatalf("second reset: %v", err) }
	defer second.Body.Close()
	if second.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected second reset 401 (token consumed), got %d", second.StatusCode)
	}
}

func TestResetPasswordRejectsBadToken(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	resetBody, _ := json.Marshal(model.ResetPasswordRequest{Token: "deadbeef", Password: "new-pw"})
	res, err := http.Post(ts.URL+"/auth/reset-password", "application/json", bytes.NewReader(resetBody))
	if err != nil { t.Fatalf("reset: %v", err) }
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown reset token, got %d", res.StatusCode)
	}
}
