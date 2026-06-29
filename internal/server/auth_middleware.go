package server

import (
	"context"
	"net/http"
	"strings"

	"go-lang/internal/auth"
	"go-lang/internal/model"
	"go-lang/internal/response"
)

const userIDCtxKey ctxKey = 100

func RequireAuth(issuer *auth.TokenIssuer, blacklist *auth.Blacklist) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			if raw == "" {
				unauthorized(w, "missing bearer token")
				return
			}

			parts := strings.SplitN(raw, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				unauthorized(w, "missing or invalid bearer token")
				return
			}

			token := strings.TrimSpace(parts[1])
			claims, err := issuer.Parse(token)
			if err != nil {
				unauthorized(w, "missing or invalid bearer token")
				return
			}

			_, jti, err := issuer.RawClaims(token)
			if err != nil {
				unauthorized(w, "missing or invalid bearer token")
				return
			}
			if blacklist != nil && blacklist.IsRevoked(jti) {
				unauthorized(w, "missing or invalid bearer token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDCtxKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userIDCtxKey).(int64)
	return v, ok
}

func unauthorized(w http.ResponseWriter, message string) {
	response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, message, nil)
}
