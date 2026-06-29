package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"go-lang/internal/model"
	"go-lang/internal/response"
)

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
						slog.Any("panic", rec),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("request_id", RequestIDFromContext(r.Context())),
						slog.String("stack", string(debug.Stack())),
					)
					response.InternalError(w, model.ErrorCodeInternal, "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
