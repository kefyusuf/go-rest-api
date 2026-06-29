package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AudienceAccess  = "go-lang:access"
	AudienceRefresh = "go-lang:refresh"
)

var (
	ErrTokenInvalid = errors.New("token is invalid")
	ErrTokenExpired = errors.New("token is expired")
)

type TokenKind string

const (
	KindAccess  TokenKind = "access"
	KindRefresh TokenKind = "refresh"
)

type Claims struct {
	Subject int64
	Issuer  string
	Kind    TokenKind
}

type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
	issuer string
	kind   TokenKind
	aud    string
}

func NewTokenIssuer(secret string, ttl time.Duration, issuer string, kind TokenKind) (*TokenIssuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("token ttl must be positive")
	}
	if kind != KindAccess && kind != KindRefresh {
		return nil, fmt.Errorf("unknown token kind %q", kind)
	}

	aud := AudienceAccess
	if kind == KindRefresh {
		aud = AudienceRefresh
	}

	return &TokenIssuer{
		secret: []byte(secret),
		ttl:    ttl,
		issuer: issuer,
		kind:   kind,
		aud:    aud,
	}, nil
}

func (i *TokenIssuer) TTL() time.Duration {
	return i.ttl
}

func (i *TokenIssuer) Kind() TokenKind {
	return i.kind
}

func (i *TokenIssuer) Issue(subject int64) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(i.ttl)
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(subject, 10),
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
		"aud": i.aud,
		"jti": newJTI(),
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

	if aud, _ := claims["aud"].(string); aud != i.aud {
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

	out := Claims{Subject: sub, Kind: i.kind}
	if iss, ok := claims["iss"].(string); ok {
		out.Issuer = iss
	}
	return out, nil
}

func (i *TokenIssuer) RawClaims(raw string) (jwt.MapClaims, string, error) {
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
		}
		return i.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, "", ErrTokenInvalid
	}
	if !token.Valid {
		return nil, "", ErrTokenInvalid
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, "", ErrTokenInvalid
	}
	if aud, _ := claims["aud"].(string); aud != i.aud {
		return nil, "", ErrTokenInvalid
	}
	jti, _ := claims["jti"].(string)
	return claims, jti, nil
}
