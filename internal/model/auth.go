package model

import "time"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken       string    `json:"accessToken"`
	RefreshToken      string    `json:"refreshToken"`
	TokenType         string    `json:"tokenType"`
	ExpiresIn         int64     `json:"expiresIn"`
	ExpiresAt         time.Time `json:"expiresAt"`
	RefreshExpiresAt  time.Time `json:"refreshExpiresAt"`
	User              User      `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ForgotPasswordResponse struct {
	Accepted  bool      `json:"accepted"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	ResetURL  string    `json:"resetUrl,omitempty"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type TokenClaims struct {
	Subject string `json:"sub"`
	Issuer  string `json:"iss,omitempty"`
}
