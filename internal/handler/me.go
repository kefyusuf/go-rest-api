package handler

import (
	"errors"
	"net/http"

	"go-lang/internal/model"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

type MeHandler struct {
	store store.UserStore
}

func NewMeHandler(userStore store.UserStore) MeHandler {
	return MeHandler{store: userStore}
}

// Me godoc
// @Summary Current user
// @Description Returns the user behind a valid bearer token
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.User
// @Failure 401 {object} model.ErrorResponse
// @Router /me [get]
func (h MeHandler) Me(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	user, err := h.store.GetByID(int(userID))
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "user no longer exists", nil)
			return
		}
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}

	user.PasswordHash = ""
	response.JSON(w, http.StatusOK, user)
}
