package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"go-lang/internal/model"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

func TestUsersCRUDFlow(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	created := createUser(t, ts.URL, model.CreateUserRequest{
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
	})

	if created.ID != 1 {
		t.Fatalf("expected created user id to be 1, got %d", created.ID)
	}

	fetched := getUser(t, ts.URL, created.ID)
	if fetched.Name != created.Name || fetched.Email != created.Email {
		t.Fatalf("fetched user mismatch: got %+v want %+v", fetched, created)
	}

	updated := updateUser(t, ts.URL, created.ID, model.UpdateUserRequest{
		Name:  "Ada Byron",
		Email: "ada.byron@example.com",
	})
	if updated.Name != "Ada Byron" || updated.Email != "ada.byron@example.com" {
		t.Fatalf("updated user mismatch: got %+v", updated)
	}

	users := listUsers(t, ts.URL)
	if len(users) != 1 {
		t.Fatalf("expected 1 user in list, got %d", len(users))
	}
	if users[0].Name != updated.Name || users[0].Email != updated.Email {
		t.Fatalf("listed user mismatch: got %+v want %+v", users[0], updated)
	}

	deleteUser(t, ts.URL, created.ID)

	res, err := http.Get(ts.URL + "/users/1")
	if err != nil {
		t.Fatalf("get deleted user failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error != "user not found" {
		t.Fatalf("expected user not found error, got %q", errorResponse.Error)
	}
}

func TestUsersDuplicateEmailReturnsConflict(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	createUser(t, ts.URL, model.CreateUserRequest{
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
	})

	body := mustJSON(t, model.CreateUserRequest{
		Name:  "Grace Hopper",
		Email: "ada@example.com",
	})

	res, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("duplicate create request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error != "email already exists" {
		t.Fatalf("expected duplicate email error, got %q", errorResponse.Error)
	}
}

func TestUsersValidationErrors(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader(mustJSON(t, map[string]string{
		"name":  "",
		"email": "",
	})))
	if err != nil {
		t.Fatalf("invalid create request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid create, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error != "name and email are required" {
		t.Fatalf("expected validation error, got %q", errorResponse.Error)
	}
}

func createUser(t *testing.T, baseURL string, input model.CreateUserRequest) model.User {
	t.Helper()

	res, err := http.Post(baseURL+"/users", "application/json", bytes.NewReader(mustJSON(t, input)))
	if err != nil {
		t.Fatalf("create user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on create, got %d", res.StatusCode)
	}

	var user model.User
	decodeJSON(t, res.Body, &user)
	return user
}

func getUser(t *testing.T, baseURL string, id int) model.User {
	t.Helper()

	res, err := http.Get(baseURL + "/users/" + intToString(id))
	if err != nil {
		t.Fatalf("get user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d", res.StatusCode)
	}

	var user model.User
	decodeJSON(t, res.Body, &user)
	return user
}

func listUsers(t *testing.T, baseURL string) []model.User {
	t.Helper()

	res, err := http.Get(baseURL + "/users")
	if err != nil {
		t.Fatalf("list users request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on list, got %d", res.StatusCode)
	}

	var users []model.User
	decodeJSON(t, res.Body, &users)
	return users
}

func updateUser(t *testing.T, baseURL string, id int, input model.UpdateUserRequest) model.User {
	t.Helper()

	req, err := http.NewRequest(http.MethodPut, baseURL+"/users/"+intToString(id), bytes.NewReader(mustJSON(t, input)))
	if err != nil {
		t.Fatalf("build update request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d", res.StatusCode)
	}

	var user model.User
	decodeJSON(t, res.Body, &user)
	return user
}

func deleteUser(t *testing.T, baseURL string, id int) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/users/"+intToString(id), nil)
	if err != nil {
		t.Fatalf("build delete request failed: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete user request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", res.StatusCode)
	}

	var message model.MessageResponse
	decodeJSON(t, res.Body, &message)
	if message.Message != "user deleted" {
		t.Fatalf("expected delete message, got %q", message.Message)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	return data
}

func decodeJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()

	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
}

func intToString(value int) string {
	return strconv.Itoa(value)
}
