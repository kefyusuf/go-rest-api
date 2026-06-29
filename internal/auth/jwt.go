package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid = errors.New("token is invalid")
	ErrTokenExpired = errors.New("token is expired")
)

type Claims struct {
	Subject int64  `json:"sub"`
	Issuer  string `json:"iss,omitempty"`
}

type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

func NewTokenIssuer(secret string, ttl time.Duration, issuer string) (*TokenIssuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("token ttl must be positive")
	}
	return &TokenIssuer{secret: []byte(secret), ttl: ttl, issuer: issuer}, nil
}

func (i *TokenIssuer) TTL() time.Duration {
	return i.ttl
}

func (i *TokenIssuer) Issue(subject int64) (string, time.Time, error) {
	expiresAt := time.Now().Add(i.ttl)
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(subject, 10),
		"iat": time.Now().Unix(),
		"exp": expiresAt.Unix(),
	}
	if i.issuer != "" {
		claims["iss"] = i.issuer
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (i *TokenIssuer) Parse(raw string) (Claims, error) {
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
		}
		return i.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Claims{}, ErrTokenExpired
		}
		return Claims{}, ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return Claims{}, ErrTokenInvalid
	}

	subRaw, ok := claims["sub"].(string)
	if !ok {
		return Claims{}, ErrTokenInvalid
	}
	sub, err := strconv.ParseInt(subRaw, 10, 64)
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}

	out := Claims{Subject: sub}
	if iss, ok := claims["iss"].(string); ok {
		out.Issuer = iss
	}
	return out, nil
}
