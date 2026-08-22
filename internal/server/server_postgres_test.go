package server_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"go-lang/internal/auth"
	"go-lang/internal/database"
	"go-lang/internal/model"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

func TestUsersCRUDFlowWithPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	db := openTestDB(t, databaseURL)
	defer db.Close()
	resetUsersTable(t, db)

	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("run migrations failed: %v", err)
	}

	access, _ := auth.NewTokenIssuer(testJWTSecret, 15*time.Minute, "test", auth.KindAccess)
	_ = access
	app := server.New(store.NewPostgresUserStore(db), newTestLogger(), server.Options{
		TokenIssuer:     access,
		BcryptCost:      4,
	})
	ts := httptest.NewServer(app)
	defer ts.Close()

	bearer := postgresBearer(t, ts.URL)

	created := createPostgresUser(t, ts.URL, bearer, model.CreateUserRequest{
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
		Password: "correct-password",
	})

	fetched := getPostgresUser(t, ts.URL, bearer, created.ID)
	if fetched.Name != created.Name || fetched.Email != created.Email {
		t.Fatalf("fetched user mismatch: got %+v want %+v", fetched, created)
	}

	updated := updatePostgresUser(t, ts.URL, bearer, created.ID, model.UpdateUserRequest{
		Name:  "Ada Byron",
		Email: "ada.byron@example.com",
	})
	if updated.Name != "Ada Byron" || updated.Email != "ada.byron@example.com" {
		t.Fatalf("updated user mismatch: got %+v", updated)
	}

	deletePostgresUser(t, ts.URL, bearer, created.ID)

	res, err := http.NewRequest(http.MethodGet, ts.URL+"/users/"+intToString(created.ID), nil)
	if err != nil {
		t.Fatalf("build get deleted user failed: %v", err)
	}
	res.Header.Set("Authorization", "Bearer "+bearer)

	res2, err := http.DefaultClient.Do(res)
	if err != nil {
		t.Fatalf("get deleted user failed: %v", err)
	}
	defer res2.Body.Close()

	if res2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", res2.StatusCode)
	}
}

func TestUsersDuplicateEmailWithPostgresReturnsConflict(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	db := openTestDB(t, databaseURL)
	defer db.Close()
	resetUsersTable(t, db)

	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("run migrations failed: %v", err)
	}

	access, _ := auth.NewTokenIssuer(testJWTSecret, 15*time.Minute, "test", auth.KindAccess)
	_ = access
	app := server.New(store.NewPostgresUserStore(db), newTestLogger(), server.Options{
		TokenIssuer:     access,
		BcryptCost:      4,
	})
	ts := httptest.NewServer(app)
	defer ts.Close()

	bearer := postgresBearer(t, ts.URL)

	createPostgresUser(t, ts.URL, bearer, model.CreateUserRequest{
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
		Password: "correct-password",
	})

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/users", bytes.NewReader(mustPostgresJSON(t, model.CreateUserRequest{
		Name:  "Grace Hopper",
		Email: "ada@example.com",
		Password: "correct-password",
	})))
	if err != nil {
		t.Fatalf("build duplicate create request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("duplicate create request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodePostgresJSON(t, res.Body, &errorResponse)
	if errorResponse.Error.Code != model.ErrorCodeConflict {
		t.Fatalf("expected CONFLICT error code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "email already exists" {
		t.Fatalf("expected duplicate email error, got %q", errorResponse.Error.Message)
	}
}

func openTestDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("ping postgres failed: %v", err)
	}

	return db
}

func resetUsersTable(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec(`DROP TABLE IF EXISTS users`); err != nil {
		t.Fatalf("drop users table failed: %v", err)
	}
}

// postgresBearer registers a bootstrap user through the public register
// endpoint and returns an access token for calling the protected users CRUD.
func postgresBearer(t *testing.T, baseURL string) string {
	t.Helper()

	body := mustPostgresJSON(t, model.CreateUserRequest{
		Name:     "Seeder",
		Email:    "seeder@example.com",
		Password: "correct-password",
	})
	res, err := http.Post(baseURL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("bootstrap register failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on bootstrap register, got %d", res.StatusCode)
	}

	var out model.LoginResponse
	decodePostgresJSON(t, res.Body, &out)
	return out.AccessToken
}

func createPostgresUser(t *testing.T, baseURL, bearer string, input model.CreateUserRequest) model.User {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/users", bytes.NewReader(mustPostgresJSON(t, input)))
	if err != nil {
		t.Fatalf("build create user request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on create, got %d", res.StatusCode)
	}

	var user model.User
	decodePostgresJSON(t, res.Body, &user)
	return user
}

func getPostgresUser(t *testing.T, baseURL, bearer string, id int) model.User {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+"/users/"+intToString(id), nil)
	if err != nil {
		t.Fatalf("build get user request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d", res.StatusCode)
	}

	var user model.User
	decodePostgresJSON(t, res.Body, &user)
	return user
}

func updatePostgresUser(t *testing.T, baseURL, bearer string, id int, input model.UpdateUserRequest) model.User {
	t.Helper()

	req, err := http.NewRequest(http.MethodPut, baseURL+"/users/"+intToString(id), bytes.NewReader(mustPostgresJSON(t, input)))
	if err != nil {
		t.Fatalf("build update request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d", res.StatusCode)
	}

	var user model.User
	decodePostgresJSON(t, res.Body, &user)
	return user
}

func deletePostgresUser(t *testing.T, baseURL, bearer string, id int) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/users/"+intToString(id), nil)
	if err != nil {
		t.Fatalf("build delete request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", res.StatusCode)
	}
}

func mustPostgresJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	return data
}

func decodePostgresJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()

	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
}
