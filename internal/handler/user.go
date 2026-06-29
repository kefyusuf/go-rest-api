package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"go-lang/internal/auth"
	"go-lang/internal/model"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

type UserHandler struct {
	store      store.UserStore
	bcryptCost int
}

func NewUserHandler(store store.UserStore, bcryptCost int) UserHandler {
	return UserHandler{store: store, bcryptCost: bcryptCost}
}

// ListUsers godoc
// @Summary List users
// @Description Lists all users
// @Tags users
// @Produce json
// @Success 200 {object} model.ListUsersResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users [get]
func (h UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	users, err := h.store.List()
	if err != nil {
		handleUserStoreError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, model.ListUsersResponse{
		Data: users,
		Meta: model.ListUsersMeta{
			NextCursor: nil,
			Limit:      20,
		},
	})
}

// CreateUser godoc
// @Summary Create user
// @Description Creates a new user
// @Tags users
// @Accept json
// @Produce json
// @Param user body model.CreateUserRequest true "New user"
// @Success 201 {object} model.User
// @Failure 400 {object} model.ErrorResponse
// @Failure 405 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Failure 415 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users [post]
func (h UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.UnsupportedMediaType(w, model.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var input model.CreateUserRequest
	if message, ok := decodeJSONBody(r, &input); !ok {
		response.BadRequest(w, model.ErrorCodeBadRequest, message, nil)
		return
	}

	if details := validateUserInput(input.Name, input.Email, input.Password); details != nil {
		response.BadRequest(w, model.ErrorCodeValidation, "validation failed", details)
		return
	}

	hashed, err := auth.HashPassword(input.Password, h.bcryptCost)
	if err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}
	input.Password = hashed

	user, err := h.store.Create(input)
	if err != nil {
		handleUserStoreError(w, err)
		return
	}

	user.PasswordHash = ""
	response.JSON(w, http.StatusCreated, user)
}

// GetUserByID godoc
// @Summary Get user by ID
// @Description Returns a user by id
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} model.User
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 405 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{id} [get]
func (h UserHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	id, err := userIDFromPath(r.URL.Path)
	if err != nil {
		response.BadRequest(w, model.ErrorCodeBadRequest, "invalid user id", nil)
		return
	}

	user, err := h.store.GetByID(id)
	if err != nil {
		handleUserStoreError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, user)
}

// UpdateUser godoc
// @Summary Update user
// @Description Updates an existing user
// @Tags users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param user body model.UpdateUserRequest true "Updated user"
// @Success 200 {object} model.User
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 405 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Failure 415 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{id} [put]
func (h UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	id, err := userIDFromPath(r.URL.Path)
	if err != nil {
		response.BadRequest(w, model.ErrorCodeBadRequest, "invalid user id", nil)
		return
	}

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.UnsupportedMediaType(w, model.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var input model.UpdateUserRequest
	if message, ok := decodeJSONBody(r, &input); !ok {
		response.BadRequest(w, model.ErrorCodeBadRequest, message, nil)
		return
	}

	if details := validateUpdateUserInput(input.Name, input.Email); details != nil {
		response.BadRequest(w, model.ErrorCodeValidation, "validation failed", details)
		return
	}

	user, err := h.store.Update(id, input)
	if err != nil {
		handleUserStoreError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Delete user
// @Description Deletes a user
// @Tags users
// @Param id path int true "User ID"
// @Success 204
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 405 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{id} [delete]
func (h UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	id, err := userIDFromPath(r.URL.Path)
	if err != nil {
		response.BadRequest(w, model.ErrorCodeBadRequest, "invalid user id", nil)
		return
	}

	if err := h.store.Delete(id); err != nil {
		handleUserStoreError(w, err)
		return
	}

	response.NoContent(w)
}

func userIDFromPath(path string) (int, error) {
	idText := strings.TrimPrefix(path, "/users/")
	if idText == "" || strings.Contains(idText, "/") {
		return 0, errors.New("invalid user id")
	}

	id, err := strconv.Atoi(idText)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func validateUserInput(name, email, password string) map[string][]string {
	details := make(map[string][]string)

	if strings.TrimSpace(name) == "" {
		details["name"] = append(details["name"], "required")
	}

	if strings.TrimSpace(email) == "" {
		details["email"] = append(details["email"], "required")
	}

	if password == "" {
		details["password"] = append(details["password"], "required")
	}

	if len(details) == 0 {
		return nil
	}

	return details
}

func validateUpdateUserInput(name, email string) map[string][]string {
	details := make(map[string][]string)

	if strings.TrimSpace(name) == "" {
		details["name"] = append(details["name"], "required")
	}

	if strings.TrimSpace(email) == "" {
		details["email"] = append(details["email"], "required")
	}

	if len(details) == 0 {
		return nil
	}

	return details
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	return mediaType == "application/json"
}

func decodeJSONBody(r *http.Request, target any) (string, bool) {
	if r.Body == nil {
		return "request body is required", false
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return "request body is required", false
		}

		return "invalid request body", false
	}

	return "", true
}

func handleUserStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrUserNotFound) {
		response.NotFound(w, model.ErrorCodeNotFound, "user not found")
		return
	}

	if errors.Is(err, store.ErrEmailAlreadyExists) {
		response.Conflict(w, model.ErrorCodeConflict, "email already exists")
		return
	}

	response.InternalError(w, model.ErrorCodeInternal, "internal server error")
}
