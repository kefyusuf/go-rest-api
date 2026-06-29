package model

import "time"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresIn   int64     `json:"expiresIn"`
	ExpiresAt   time.Time `json:"expiresAt"`
	User        User      `json:"user"`
}

type TokenClaims struct {
	Subject string `json:"sub"`
	Issuer  string `json:"iss,omitempty"`
}
