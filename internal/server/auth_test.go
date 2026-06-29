package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go-lang/internal/auth"
	"go-lang/internal/model"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

const testJWTSecret = "this-is-a-test-jwt-secret-of-at-least-32-bytes"

func jwtMintExpired(t *testing.T, secret string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(1, 10),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"exp": time.Now().Add(-time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

func newAuthApp(t *testing.T) (*httptest.Server, *auth.TokenIssuer, *auth.TokenIssuer, auth.Blacklist, store.UserStore) {
	t.Helper()
	userStore := store.NewMemoryUserStore()
	issuer, err := auth.NewTokenIssuer(testJWTSecret, 15*time.Minute, "test", auth.KindAccess)
	if err != nil {
		t.Fatalf("access issuer: %v", err)
	}
	refresh, err := auth.NewTokenIssuer(testJWTSecret, 24*time.Hour, "test", auth.KindRefresh)
	if err != nil {
		t.Fatalf("refresh issuer: %v", err)
	}
	blacklist := auth.NewBlacklist()

	app := server.New(userStore, newTestLogger(), server.Options{
		BcryptCost:    4,
		TokenIssuer:   issuer,
		RefreshIssuer: refresh,
		Blacklist:     blacklist,
	})
	ts := httptest.NewServer(app)
	return ts, issuer, refresh, blacklist, userStore
}

func seedUser(t *testing.T, ts *httptest.Server, name, email, password string) model.User {
	t.Helper()
	body, err := json.Marshal(model.CreateUserRequest{
		Name:     name,
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	res, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on seed, got %d", res.StatusCode)
	}

	var user model.User
	decodeJSON(t, res.Body, &user)
	return user
}

func TestLoginHappyPath(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	seedUser(t, ts, "Ada Lovelace", "ada@example.com", "correct-horse-battery-staple")

	body, _ := json.Marshal(model.LoginRequest{
		Email:    "ada@example.com",
		Password: "correct-horse-battery-staple",
	})
	res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var loginRes model.LoginResponse
	decodeJSON(t, res.Body, &loginRes)
	if loginRes.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if loginRes.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", loginRes.TokenType)
	}
	if loginRes.ExpiresIn <= 0 {
		t.Fatalf("expected positive expiresIn, got %d", loginRes.ExpiresIn)
	}
	if loginRes.User.ID == 0 {
		t.Fatal("expected user id in response")
	}
	if loginRes.User.PasswordHash != "" {
		t.Fatalf("password hash leaked in response: %q", loginRes.User.PasswordHash)
	}
	if loginRes.User.Email != "ada@example.com" {
		t.Fatalf("expected ada@example.com in response, got %q", loginRes.User.Email)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	seedUser(t, ts, "Ada", "ada@example.com", "right-password")

	body, _ := json.Marshal(model.LoginRequest{
		Email:    "ada@example.com",
		Password: "wrong-password",
	})
	res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	var payload model.ErrorResponse
	decodeJSON(t, res.Body, &payload)
	if payload.Error.Code != model.ErrorCodeUnauthorized {
		t.Fatalf("expected UNAUTHORIZED, got %q", payload.Error.Code)
	}
	if payload.Error.Message != "invalid email or password" {
		t.Fatalf("expected generic message, got %q", payload.Error.Message)
	}
}

func TestLoginRejectsUnknownUser(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	body, _ := json.Marshal(model.LoginRequest{
		Email:    "ghost@example.com",
		Password: "any-password",
	})
	res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	var payload model.ErrorResponse
	decodeJSON(t, res.Body, &payload)
	if payload.Error.Message != "invalid email or password" {
		t.Fatalf("expected generic message, got %q", payload.Error.Message)
	}
}

func TestLoginValidation(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	body, _ := json.Marshal(model.LoginRequest{Email: "", Password: ""})
	res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	var payload model.ErrorResponse
	decodeJSON(t, res.Body, &payload)
	if payload.Error.Code != model.ErrorCodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %q", payload.Error.Code)
	}
	if payload.Error.Details == nil {
		t.Fatal("expected validation details")
	}
}

func TestLoginMalformedJSON(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader([]byte("{")))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

func TestLoginWrongContentType(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/auth/login", "text/plain", bytes.NewReader([]byte("ok")))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", res.StatusCode)
	}
}

func TestMeWithValidToken(t *testing.T) {
	ts, issuer, _, _, _ := newAuthApp(t)
	defer ts.Close()

	seedUser(t, ts, "Ada", "ada@example.com", "pw")
	token, _, err := issuer.Issue(1)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var user model.User
	decodeJSON(t, res.Body, &user)
	if user.Email != "ada@example.com" {
		t.Fatalf("expected ada@example.com, got %q", user.Email)
	}
	if user.PasswordHash != "" {
		t.Fatalf("password hash leaked: %q", user.PasswordHash)
	}
}

func TestMeRejectsMissingHeader(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/me")
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func TestMeRejectsMalformedHeader(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/me", nil)
	req.Header.Set("Authorization", "Token abc.def.ghi")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func TestMeRejectsExpiredToken(t *testing.T) {
	ts, _, _, _, _ := newAuthApp(t)
	defer ts.Close()

	expiredToken := jwtMintExpired(t, testJWTSecret)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/me", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func TestMeRejectsTamperedToken(t *testing.T) {
	ts, issuer, _, _, _ := newAuthApp(t)
	defer ts.Close()

	seedUser(t, ts, "Ada", "ada@example.com", "pw")
	token, _, err := issuer.Issue(1)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	tampered := token[:len(token)-2] + "AA"

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/me", nil)
	req.Header.Set("Authorization", "Bearer "+tampered)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}
