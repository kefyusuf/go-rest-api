package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go-lang/internal/model"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

type UserHandler struct {
	store store.UserStore
}

func NewUserHandler(store store.UserStore) UserHandler {
	return UserHandler{store: store}
}

// ListUsers godoc
// @Summary List users
// @Description Tüm kullanıcıları listeler
// @Tags users
// @Produce json
// @Success 200 {array} model.User
// @Failure 500 {object} model.ErrorResponse
// @Router /users [get]
func (h UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.JSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "method not allowed"})
		return
	}

	users, err := h.store.List()
	if err != nil {
		handleUserStoreError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, users)
}

// CreateUser godoc
// @Summary Create user
// @Description Yeni kullanıcı oluşturur
// @Tags users
// @Accept json
// @Produce json
// @Param user body model.CreateUserRequest true "New user"
// @Success 201 {object} model.User
// @Failure 400 {object} model.ErrorResponse
// @Failure 405 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users [post]
func (h UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.JSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "method not allowed"})
		return
	}

	var input model.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.JSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid request body"})
		return
	}

	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Email) == "" {
		response.JSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "name and email are required"})
		return
	}

	user, err := h.store.Create(input)
	if err != nil {
		handleUserStoreError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, user)
}

// GetUserByID godoc
// @Summary Get user by ID
// @Description ID ile kullanıcı getirir
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
		response.JSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "method not allowed"})
		return
	}

	id, err := userIDFromPath(r.URL.Path)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid user id"})
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
// @Description Var olan kullanıcıyı günceller
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
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{id} [put]
func (h UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		response.JSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "method not allowed"})
		return
	}

	id, err := userIDFromPath(r.URL.Path)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid user id"})
		return
	}

	var input model.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.JSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid request body"})
		return
	}

	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Email) == "" {
		response.JSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "name and email are required"})
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
// @Description Kullanıcıyı siler
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} model.MessageResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 405 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{id} [delete]
func (h UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.JSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "method not allowed"})
		return
	}

	id, err := userIDFromPath(r.URL.Path)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid user id"})
		return
	}

	if err := h.store.Delete(id); err != nil {
		handleUserStoreError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, model.MessageResponse{Message: "user deleted"})
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

func handleUserStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrUserNotFound) {
		response.JSON(w, http.StatusNotFound, model.ErrorResponse{Error: "user not found"})
		return
	}

	if errors.Is(err, store.ErrEmailAlreadyExists) {
		response.JSON(w, http.StatusConflict, model.ErrorResponse{Error: "email already exists"})
		return
	}

	response.JSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "internal server error"})
}
