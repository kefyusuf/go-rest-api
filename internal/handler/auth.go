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
	store          store.UserStore
	issuer         *auth.TokenIssuer
	refreshIssuer  *auth.TokenIssuer
	blacklist      auth.Blacklist
	bcryptCost     int
	resetTokens    *resetTokenStore
	now            func() time.Time
	resetTokenTTL  time.Duration
	forgotDisabled bool
}

type AuthHandlerOptions struct {
	BcryptCost    int
	ResetTokenTTL time.Duration
}

func NewAuthHandler(userStore store.UserStore, issuer *auth.TokenIssuer, refreshIssuer *auth.TokenIssuer, blacklist auth.Blacklist, opts AuthHandlerOptions) AuthHandler {
	if opts.ResetTokenTTL <= 0 {
		opts.ResetTokenTTL = time.Hour
	}
	return AuthHandler{
		store:         userStore,
		issuer:        issuer,
		refreshIssuer: refreshIssuer,
		blacklist:     blacklist,
		bcryptCost:    opts.BcryptCost,
		resetTokens:   newResetTokenStore(),
		now:           time.Now,
		resetTokenTTL: opts.ResetTokenTTL,
	}
}

// Register godoc
// @Summary Register
// @Description Creates a new user and returns a JWT access token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body model.CreateUserRequest true "New account"
// @Success 201 {object} model.LoginResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Failure 415 {object} model.ErrorResponse
// @Router /auth/register [post]
func (h AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
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

	email := strings.ToLower(strings.TrimSpace(input.Email))
	input.Email = email

	hashed, err := auth.HashPassword(input.Password, h.bcryptCost)
	if err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}
	input.Password = hashed

	user, err := h.store.Create(input)
	if err != nil {
		if errors.Is(err, store.ErrEmailAlreadyExists) {
			response.Conflict(w, model.ErrorCodeConflict, "email already exists")
			return
		}
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}

	token, refreshToken, expiresAt, refreshExpiresAt, err := h.issueTokens(int64(user.ID))
	if err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}
	user.PasswordHash = ""
	ttl := time.Until(expiresAt)
	if ttl < 0 {
		ttl = 0
	}
	response.JSON(w, http.StatusCreated, model.LoginResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(ttl.Seconds()),
		ExpiresAt:    expiresAt,
		User:         user,
		RefreshExpiresAt: refreshExpiresAt,
	})
}

// Login godoc
// @Summary Login
// @Description Authenticates a user with email and password, returns JWT access and refresh tokens
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
	if details := validateLoginInput(input.Email, input.Password); details != nil {
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

	token, refreshToken, expiresAt, refreshExpiresAt, err := h.issueTokens(int64(user.ID))
	if err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}
	user.PasswordHash = ""
	ttl := time.Until(expiresAt)
	if ttl < 0 {
		ttl = 0
	}
	response.JSON(w, http.StatusOK, model.LoginResponse{
		AccessToken:       token,
		RefreshToken:      refreshToken,
		TokenType:         "Bearer",
		ExpiresIn:         int64(ttl.Seconds()),
		ExpiresAt:         expiresAt,
		User:              user,
		RefreshExpiresAt: refreshExpiresAt,
	})
}

// Logout godoc
// @Summary Logout
// @Description Revokes the bearer token used in this request
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} model.ErrorResponse
// @Router /auth/logout [post]
func (h AuthHandler) Logout(w http.ResponseWriter, r *http.Request, accessToken string) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	rawClaims, jti, err := h.issuer.RawClaims(accessToken)
	if err != nil {
		unauthorizedFromAuth(w)
		return
	}
	expFloat, ok := rawClaims["exp"].(float64)
	if !ok {
		unauthorizedFromAuth(w)
		return
	}
	expiresAt := time.Unix(int64(expFloat), 0)
	h.blacklist.Revoke(jti, expiresAt)
	response.NoContent(w)
}

// Refresh godoc
// @Summary Refresh
// @Description Trades a valid refresh token for a new access token (and a new refresh token)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.RefreshRequest true "Refresh token"
// @Success 200 {object} model.LoginResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Router /auth/refresh [post]
func (h AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.UnsupportedMediaType(w, model.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var input model.RefreshRequest
	if message, ok := decodeJSONBody(r, &input); !ok {
		response.BadRequest(w, model.ErrorCodeBadRequest, message, nil)
		return
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		response.BadRequest(w, model.ErrorCodeValidation, "validation failed", map[string][]string{
			"refreshToken": {"required"},
		})
		return
	}

	claims, err := h.refreshIssuer.Parse(input.RefreshToken)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "invalid refresh token", nil)
		return
	}
	rawClaims, jti, err := h.refreshIssuer.RawClaims(input.RefreshToken)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "invalid refresh token", nil)
		return
	}
	if h.blacklist.IsRevoked(jti) {
		response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "invalid refresh token", nil)
		return
	}
	expFloat, _ := rawClaims["exp"].(float64)
	refreshExpiresAt := time.Unix(int64(expFloat), 0)
	h.blacklist.Revoke(jti, refreshExpiresAt)

	token, newRefresh, expiresAt, newRefreshExpiresAt, err := h.issueTokens(claims.Subject)
	if err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}

	user, err := h.store.GetByID(int(claims.Subject))
	if err != nil {
		response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "user no longer exists", nil)
		return
	}
	user.PasswordHash = ""
	ttl := time.Until(expiresAt)
	if ttl < 0 {
		ttl = 0
	}
	response.JSON(w, http.StatusOK, model.LoginResponse{
		AccessToken:       token,
		RefreshToken:      newRefresh,
		TokenType:         "Bearer",
		ExpiresIn:         int64(ttl.Seconds()),
		ExpiresAt:         expiresAt,
		User:              user,
		RefreshExpiresAt: newRefreshExpiresAt,
	})
}

// ForgotPassword godoc
// @Summary Forgot password
// @Description Always returns 202 to avoid leaking which emails are registered. In this
// @Description in-memory starter the response includes the reset token directly; a
// @Description production deployment would email it to the user instead.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.ForgotPasswordRequest true "Forgot password request"
// @Success 202 {object} model.ForgotPasswordResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 415 {object} model.ErrorResponse
// @Router /auth/forgot-password [post]
func (h AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.UnsupportedMediaType(w, model.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var input model.ForgotPasswordRequest
	if message, ok := decodeJSONBody(r, &input); !ok {
		response.BadRequest(w, model.ErrorCodeBadRequest, message, nil)
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		response.BadRequest(w, model.ErrorCodeValidation, "validation failed", map[string][]string{
			"email": {"required"},
		})
		return
	}

	user, err := h.store.GetByEmail(email)
	if err != nil {
		response.JSON(w, http.StatusAccepted, model.ForgotPasswordResponse{
			Accepted: true,
		})
		return
	}
	token, expiresAt := h.resetTokens.Issue(int64(user.ID), h.now, h.resetTokenTTL)

	response.JSON(w, http.StatusAccepted, model.ForgotPasswordResponse{
		Accepted:   true,
		Token:      token,
		ExpiresAt:  expiresAt,
		ResetURL:   "/auth/reset-password",
	})
}

// ResetPassword godoc
// @Summary Reset password
// @Description Resets a user's password using a valid reset token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.ResetPasswordRequest true "Reset password request"
// @Success 204
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 415 {object} model.ErrorResponse
// @Router /auth/reset-password [post]
func (h AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.UnsupportedMediaType(w, model.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var input model.ResetPasswordRequest
	if message, ok := decodeJSONBody(r, &input); !ok {
		response.BadRequest(w, model.ErrorCodeBadRequest, message, nil)
		return
	}
	details := map[string][]string{}
	if strings.TrimSpace(input.Token) == "" {
		details["token"] = append(details["token"], "required")
	}
	if input.Password == "" {
		details["password"] = append(details["password"], "required")
	}
	if len(details) > 0 {
		response.BadRequest(w, model.ErrorCodeValidation, "validation failed", details)
		return
	}

	userID, ok := h.resetTokens.Consume(input.Token, h.now)
	if !ok {
		response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "invalid or expired reset token", nil)
		return
	}

	hashed, err := auth.HashPassword(input.Password, h.bcryptCost)
	if err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}
	if err := h.store.UpdatePassword(int(userID), hashed); err != nil {
		response.InternalError(w, model.ErrorCodeInternal, "internal server error")
		return
	}
	response.NoContent(w)
}

func (h AuthHandler) issueTokens(userID int64) (string, string, time.Time, time.Time, error) {
	token, expiresAt, err := h.issuer.Issue(userID)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	refresh, refreshExpiresAt, err := h.refreshIssuer.Issue(userID)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	return token, refresh, expiresAt, refreshExpiresAt, nil
}

func validateLoginInput(email, password string) map[string][]string {
	details := make(map[string][]string)
	if email == "" {
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

func unauthorizedFromAuth(w http.ResponseWriter) {
	response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "missing or invalid bearer token", nil)
}
