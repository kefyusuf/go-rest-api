package server_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"go-lang/internal/model"
)

func TestUsersRoutesRejectAnonymousCalls(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	registerUser(t, ts, "Ada", "ada@example.com", "pw")

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"list", http.MethodGet, "/users", ""},
		{"get by id", http.MethodGet, "/users/1", ""},
		{"create", http.MethodPost, "/users", `{"name":"Bob","email":"bob@example.com","password":"pw"}`},
		{"update", http.MethodPut, "/users/1", `{"name":"Ada Byron","email":"ada.byron@example.com"}`},
		{"delete", http.MethodDelete, "/users/1", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = bytes.NewReader([]byte(tc.body))
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, body)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.method, tc.path, err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected 401 for anonymous %s %s, got %d", tc.method, tc.path, res.StatusCode)
			}

			var out model.ErrorResponse
			decodeJSON(t, res.Body, &out)
			if out.Error.Code != model.ErrorCodeUnauthorized {
				t.Fatalf("expected %q error code, got %q", model.ErrorCodeUnauthorized, out.Error.Code)
			}
		})
	}
}

func TestAuthenticatedUserCanListAndGet(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	registerUser(t, ts, "Ada", "ada@example.com", "pw")
	token := registerUser(t, ts, "Grace", "grace@example.com", "pw").AccessToken

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on authenticated list, got %d", res.StatusCode)
	}

	var list model.ListUsersResponse
	decodeJSON(t, res.Body, &list)
	if len(list.Data) != 2 {
		t.Fatalf("expected 2 users in list, got %d", len(list.Data))
	}

	get, err := http.NewRequest(http.MethodGet, ts.URL+"/users/1", nil)
	if err != nil {
		t.Fatalf("build get: %v", err)
	}
	get.Header.Set("Authorization", "Bearer "+token)
	res2, err := http.DefaultClient.Do(get)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on authenticated get, got %d", res2.StatusCode)
	}
}

func TestAuthenticatedCreateUpdateDeleteCycle(t *testing.T) {
	ts, _, _, _, _ := newSessionApp(t)
	defer ts.Close()

	token := registerUser(t, ts, "Ada", "ada@example.com", "pw").AccessToken

	createBody := mustJSON(t, model.CreateUserRequest{
		Name: "Bob", Email: "bob@example.com", Password: "pw",
	})
	authReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/users", bytes.NewReader(createBody))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("Authorization", "Bearer "+token)
	authRes, err := http.DefaultClient.Do(authReq)
	if err != nil {
		t.Fatalf("authenticated create: %v", err)
	}
	defer authRes.Body.Close()
	if authRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on authenticated create, got %d", authRes.StatusCode)
	}

	var created model.User
	decodeJSON(t, authRes.Body, &created)

	updateBody := mustJSON(t, model.UpdateUserRequest{Name: "Bob B.", Email: "bob.b@example.com"})
	update, _ := http.NewRequest(http.MethodPut, ts.URL+"/users/"+intToString(created.ID), bytes.NewReader(updateBody))
	update.Header.Set("Content-Type", "application/json")
	update.Header.Set("Authorization", "Bearer "+token)
	res3, err := http.DefaultClient.Do(update)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	res3.Body.Close()
	if res3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d", res3.StatusCode)
	}

	del, _ := http.NewRequest(http.MethodDelete, ts.URL+"/users/"+intToString(created.ID), nil)
	del.Header.Set("Authorization", "Bearer "+token)
	res4, err := http.DefaultClient.Do(del)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	res4.Body.Close()
	if res4.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", res4.StatusCode)
	}
}
