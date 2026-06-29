package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"go-lang/internal/auth"
	"go-lang/internal/model"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

type AuthHandler struct {
	store  store.UserStore
	issuer *auth.TokenIssuer
}

func NewAuthHandler(userStore store.UserStore, issuer *auth.TokenIssuer) AuthHandler {
	return AuthHandler{store: userStore, issuer: issuer}
}

// Login godoc
// @Summary Login
// @Description Authenticates a user with email and password, returns a JWT access token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body model.LoginRequest true "Login credentials"
// @Success 200 {object} model.LoginResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 415 {object} model.ErrorResponse
// @Router /auth/login [post]
func (h AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.UnsupportedMediaType(w, model.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var input model.LoginRequest
	if message, ok := decodeJSONBody(r, &input); !ok {
		response.BadRequest(w, model.ErrorCodeBadRequest, message, nil)
		return
	}

	details := validateLoginInput(input.Email, input.Password)
	if details != nil {
		response.BadRequest(w, model.ErrorCodeValidation, "validation failed", details)
		return
	}

	user, err := h.store.GetByEmail(strings.ToLower(strings.TrimSpace(input.Email)))
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "invalid email or password", nil)
			return
		}
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, input.Password); err != nil {
		response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "invalid email or password", nil)
		return
	}

	token, expiresAt, err := h.issuer.Issue(int64(user.ID))
	if err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}

	ttl := time.Until(expiresAt)
	if ttl < 0 {
		ttl = 0
	}

	user.PasswordHash = ""
	response.JSON(w, http.StatusOK, model.LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(ttl.Seconds()),
		ExpiresAt:   expiresAt,
		User:        user,
	})
}

func validateLoginInput(email, password string) map[string][]string {
	details := make(map[string][]string)

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
