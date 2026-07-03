package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"go-lang/internal/idempotency"
	"go-lang/internal/model"
	"go-lang/internal/response"
)

const idempotencyTimeout = 5 * time.Second

type IdempotencyOptions struct {
	Store idempotency.Store
	Now   func() time.Time
}

// WithIdempotency wraps a handler so the request body is hashed, the
// key is consulted first, and the cached response is replayed when
// the same key and the same body are seen again. When the key is
// missing the handler runs normally.
func WithIdempotency(opts IdempotencyOptions, next http.Handler) http.Handler {
	store := opts.Store
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(idempotency.Header)
		if key == "" || store == nil {
			next.ServeHTTP(w, r)
			return
		}

		var bodyBytes []byte
		if r.Body != nil && r.Body != http.NoBody {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				response.Error(w, http.StatusBadRequest, model.ErrorCodeBadRequest, "could not read request body", nil)
				return
			}
			bodyBytes = data
		}

		ctx, cancel := context.WithTimeout(r.Context(), idempotencyTimeout)
		defer cancel()

		rec := &captureRecorder{header: http.Header{}}

		entry, replayed, err := store.Run(ctx, key, bodyBytes, func(_ context.Context) (int, []byte, string, error) {
			doReq := r.Clone(ctx)
			doReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			doReq.ContentLength = int64(len(bodyBytes))

			next.ServeHTTP(rec, doReq)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			ct := rec.header.Get("Content-Type")
			return status, rec.body.Bytes(), ct, nil
		})

		if err != nil {
			if errors.Is(err, idempotency.ErrConflict) {
				slog.Warn("idempotency key reused with a different body",
					slog.String("key", key),
					slog.String("path", r.URL.Path),
				)
				response.Error(w, http.StatusConflict, model.ErrorCodeConflict, "idempotency key reused with a different request body", nil)
				return
			}
			slog.Error("idempotency lookup failed", slog.String("error", err.Error()))
			next.ServeHTTP(w, r)
			return
		}

		if replayed {
			slog.Info("idempotent replay",
				slog.String("key", key),
				slog.String("path", r.URL.Path),
				slog.Int("status", entry.Status),
			)
		}

		for k, v := range rec.header {
			for _, val := range v {
				w.Header().Add(k, val)
			}
		}
		if entry.ContentType != "" && w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", entry.ContentType)
		}
		if replayed {
			w.Header().Set("Idempotent-Replay", "true")
		}
		w.WriteHeader(entry.Status)
		if len(entry.Body) > 0 {
			_, _ = w.Write(entry.Body)
		}
	})
}

type captureRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *captureRecorder) Header() http.Header { return r.header }
func (r *captureRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(p)
}
func (r *captureRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}
