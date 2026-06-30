package server

import (
	"log/slog"
	"net/http"
	"strings"

	httpSwagger "github.com/swaggo/http-swagger"

	"go-lang/internal/auth"
	"go-lang/internal/events"
	"go-lang/internal/handler"
	"go-lang/internal/idempotency"
	"go-lang/internal/jobs"
	"go-lang/internal/model"
	"go-lang/internal/observability"
	"go-lang/internal/ratelimit"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

type Options struct {
	MaxBodyBytes  int64
	TokenIssuer   *auth.TokenIssuer
	RefreshIssuer *auth.TokenIssuer
	Blacklist     auth.Blacklist
	BcryptCost    int
	Metrics       *observability.Metrics
	HealthProbes  *observability.HealthProbes
	DBPinger      observability.Pinger

	GlobalLimiter ratelimit.Limiter
	AuthLimiter   ratelimit.Limiter
	CORS          CORSConfig

	IdempotencyStore idempotency.Store
	ResetTokens     handler.TokenStore
	JobQueue         jobs.Queue
	Outbox           events.Outbox
}

func New(userStore store.UserStore, logger *slog.Logger, opts Options) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	healthHandler := handler.NewHealthHandler()
	userHandler := handler.NewUserHandler(userStore, opts.BcryptCost, opts.Outbox)
	meHandler := handler.NewMeHandler(userStore)

	authLimiter := RateLimit(RateLimitConfig{Limiter: opts.AuthLimiter})
	globalLimiter := RateLimit(RateLimitConfig{Limiter: opts.GlobalLimiter})

	if opts.HealthProbes != nil {
		mux.Handle("/health/live", http.HandlerFunc(opts.HealthProbes.Liveness))
		mux.Handle("/health/ready", http.HandlerFunc(opts.HealthProbes.Readiness(opts.DBPinger)))
	}
	if opts.Metrics != nil {
		mux.Handle("/metrics", opts.Metrics.Handler())
	}

	if opts.TokenIssuer != nil && opts.RefreshIssuer != nil && opts.Blacklist != nil {
		authHandler := handler.NewAuthHandler(userStore, opts.TokenIssuer, opts.RefreshIssuer, opts.Blacklist, handler.AuthHandlerOptions{
			BcryptCost:    opts.BcryptCost,
			ResetTokens:   opts.ResetTokens,
			JobQueue:      opts.JobQueue,
		})

		mux.Handle("/auth/login", authLimiter(http.HandlerFunc(authHandler.Login)))
		mux.Handle("/auth/register", authLimiter(WithIdempotency(IdempotencyOptions{Store: opts.IdempotencyStore}, http.HandlerFunc(authHandler.Register))))
		mux.Handle("/auth/refresh", authLimiter(http.HandlerFunc(authHandler.Refresh)))
		mux.Handle("/auth/forgot-password", authLimiter(http.HandlerFunc(authHandler.ForgotPassword)))
		mux.Handle("/auth/reset-password", authLimiter(http.HandlerFunc(authHandler.ResetPassword)))
		mux.Handle("/auth/logout", RequireAuth(opts.TokenIssuer, opts.Blacklist)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			parts := strings.SplitN(raw, " ", 2)
			if len(parts) != 2 {
				unauthorized(w, "missing or invalid bearer token")
				return
			}
			authHandler.Logout(w, r, strings.TrimSpace(parts[1]))
		})))
		mux.Handle("/me", globalLimiter(RequireAuth(opts.TokenIssuer, opts.Blacklist)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "missing or invalid bearer token", nil)
				return
			}
			meHandler.Me(w, r, userID)
		}))))
	}

	mux.HandleFunc("/health", healthHandler.Check)
	mux.Handle("/users", globalLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.ListUsers(w, r)
		case http.MethodPost:
			WithIdempotency(IdempotencyOptions{Store: opts.IdempotencyStore}, http.HandlerFunc(userHandler.CreateUser)).ServeHTTP(w, r)
		default:
			response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		}
	})))
	mux.Handle("/users/", globalLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.GetUserByID(w, r)
		case http.MethodPut:
			userHandler.UpdateUser(w, r)
		case http.MethodDelete:
			userHandler.DeleteUser(w, r)
		default:
			response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		}
	})))
	mux.Handle("/swagger/", httpSwagger.WrapHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			response.NotFound(w, model.ErrorCodeNotFound, "not found")
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{
			"message": "Welcome to the Go API starter",
		})
	})

	chain := CORS(opts.CORS)(
		SecurityHeaders(
			RequestID(
				Recovery(logger)(
					AccessLog(logger)(
						MetricsMiddleware(opts.Metrics)(
							BodyLimit(opts.MaxBodyBytes)(mux),
						),
					),
				),
			),
		),
	)
	return chain
}
