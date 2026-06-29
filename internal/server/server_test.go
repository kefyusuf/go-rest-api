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

	listResponse := listUsers(t, ts.URL)
	if len(listResponse.Data) != 1 {
		t.Fatalf("expected 1 user in list, got %d", len(listResponse.Data))
	}
	if listResponse.Data[0].Name != updated.Name || listResponse.Data[0].Email != updated.Email {
		t.Fatalf("listed user mismatch: got %+v want %+v", listResponse.Data[0], updated)
	}
	if listResponse.Meta.Limit != 20 {
		t.Fatalf("expected list meta limit to be 20, got %d", listResponse.Meta.Limit)
	}
	if listResponse.Meta.NextCursor != nil {
		t.Fatalf("expected nextCursor to be nil, got %v", *listResponse.Meta.NextCursor)
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
	if errorResponse.Error.Code != model.ErrorCodeNotFound {
		t.Fatalf("expected NOT_FOUND error code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "user not found" {
		t.Fatalf("expected user not found error, got %q", errorResponse.Error.Message)
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
	if errorResponse.Error.Code != model.ErrorCodeConflict {
		t.Fatalf("expected CONFLICT error code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "email already exists" {
		t.Fatalf("expected duplicate email error, got %q", errorResponse.Error.Message)
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
	if errorResponse.Error.Code != model.ErrorCodeValidation {
		t.Fatalf("expected VALIDATION_ERROR code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "validation failed" {
		t.Fatalf("expected validation failed message, got %q", errorResponse.Error.Message)
	}
	if errorResponse.Error.Details == nil {
		t.Fatal("expected validation details, got nil")
	}

	nameDetails := errorResponse.Error.Details["name"]
	if len(nameDetails) != 1 || nameDetails[0] != "required" {
		t.Fatalf("expected name required detail, got %+v", nameDetails)
	}

	emailDetails := errorResponse.Error.Details["email"]
	if len(emailDetails) != 1 || emailDetails[0] != "required" {
		t.Fatalf("expected email required detail, got %+v", emailDetails)
	}
}

func TestUsersCreateRejectsUnsupportedMediaType(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/users", "text/plain", bytes.NewReader([]byte("not-json")))
	if err != nil {
		t.Fatalf("unsupported media type create request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for unsupported media type, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error.Code != model.ErrorCodeUnsupportedMediaType {
		t.Fatalf("expected UNSUPPORTED_MEDIA_TYPE code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "Content-Type must be application/json" {
		t.Fatalf("expected unsupported media type message, got %q", errorResponse.Error.Message)
	}
}

func TestUsersUpdateRejectsUnsupportedMediaType(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	created := createUser(t, ts.URL, model.CreateUserRequest{
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
	})

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/users/"+intToString(created.ID), bytes.NewReader([]byte("not-json")))
	if err != nil {
		t.Fatalf("build unsupported media type update request failed: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unsupported media type update request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for unsupported media type update, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error.Code != model.ErrorCodeUnsupportedMediaType {
		t.Fatalf("expected UNSUPPORTED_MEDIA_TYPE code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "Content-Type must be application/json" {
		t.Fatalf("expected unsupported media type message, got %q", errorResponse.Error.Message)
	}
}

func TestUsersCreateRejectsMalformedJSON(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader([]byte("{")))
	if err != nil {
		t.Fatalf("malformed create request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed json, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error.Code != model.ErrorCodeBadRequest {
		t.Fatalf("expected BAD_REQUEST code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "invalid request body" {
		t.Fatalf("expected invalid request body message, got %q", errorResponse.Error.Message)
	}
}

func TestUsersCreateRejectsEmptyBody(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("empty body create request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error.Code != model.ErrorCodeBadRequest {
		t.Fatalf("expected BAD_REQUEST code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "request body is required" {
		t.Fatalf("expected request body is required message, got %q", errorResponse.Error.Message)
	}
}

func TestUsersUpdateRejectsMalformedJSON(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	created := createUser(t, ts.URL, model.CreateUserRequest{
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
	})

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/users/"+intToString(created.ID), bytes.NewReader([]byte("{")))
	if err != nil {
		t.Fatalf("build malformed update request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("malformed update request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed update json, got %d", res.StatusCode)
	}

	var errorResponse model.ErrorResponse
	decodeJSON(t, res.Body, &errorResponse)
	if errorResponse.Error.Code != model.ErrorCodeBadRequest {
		t.Fatalf("expected BAD_REQUEST code, got %q", errorResponse.Error.Code)
	}
	if errorResponse.Error.Message != "invalid request body" {
		t.Fatalf("expected invalid request body message, got %q", errorResponse.Error.Message)
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

func listUsers(t *testing.T, baseURL string) model.ListUsersResponse {
	t.Helper()

	res, err := http.Get(baseURL + "/users")
	if err != nil {
		t.Fatalf("list users request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on list, got %d", res.StatusCode)
	}

	var listResponse model.ListUsersResponse
	decodeJSON(t, res.Body, &listResponse)
	return listResponse
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

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", res.StatusCode)
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

func TestMethodNotAllowedIncludesAllowHeader(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/users", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res.StatusCode)
	}
	if got := res.Header.Get("Allow"); got != "GET, POST" {
		t.Fatalf("expected Allow=GET, POST on /users, got %q", got)
	}

	req2, _ := http.NewRequest(http.MethodPatch, ts.URL+"/users/1", nil)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("patch /users/1: %v", err)
	}
	defer res2.Body.Close()

	if res2.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res2.StatusCode)
	}
	if got := res2.Header.Get("Allow"); got != "GET, PUT, DELETE" {
		t.Fatalf("expected Allow=GET, PUT, DELETE on /users/{id}, got %q", got)
	}
}

func TestCreateUserRejectsBodyWithTrailingData(t *testing.T) {
	app := server.New(store.NewMemoryUserStore())
	ts := httptest.NewServer(app)
	defer ts.Close()

	body := []byte(`{"name":"Ada","email":"ada@example.com"}{"extra":1}`)
	res, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for trailing data, got %d", res.StatusCode)
	}
}
