package server

import (
	"log/slog"
	"net/http"
	"strings"

	httpSwagger "github.com/swaggo/http-swagger"

	"go-lang/internal/auth"
	"go-lang/internal/handler"
	"go-lang/internal/model"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

type Options struct {
	MaxBodyBytes  int64
	TokenIssuer   *auth.TokenIssuer
	RefreshIssuer *auth.TokenIssuer
	Blacklist     *auth.Blacklist
	BcryptCost    int
}

func New(userStore store.UserStore, logger *slog.Logger, opts Options) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	healthHandler := handler.NewHealthHandler()
	userHandler := handler.NewUserHandler(userStore, opts.BcryptCost)
	meHandler := handler.NewMeHandler(userStore)

	if opts.TokenIssuer != nil && opts.RefreshIssuer != nil && opts.Blacklist != nil {
		authHandler := handler.NewAuthHandler(userStore, opts.TokenIssuer, opts.RefreshIssuer, opts.Blacklist, handler.AuthHandlerOptions{
			BcryptCost: opts.BcryptCost,
		})

		mux.HandleFunc("/auth/login", authHandler.Login)
		mux.HandleFunc("/auth/register", authHandler.Register)
		mux.HandleFunc("/auth/refresh", authHandler.Refresh)
		mux.HandleFunc("/auth/forgot-password", authHandler.ForgotPassword)
		mux.HandleFunc("/auth/reset-password", authHandler.ResetPassword)
		mux.Handle("/auth/logout", RequireAuth(opts.TokenIssuer, opts.Blacklist)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			parts := strings.SplitN(raw, " ", 2)
			if len(parts) != 2 {
				unauthorized(w, "missing or invalid bearer token")
				return
			}
			authHandler.Logout(w, r, strings.TrimSpace(parts[1]))
		})))
		mux.Handle("/me", RequireAuth(opts.TokenIssuer, opts.Blacklist)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, model.ErrorCodeUnauthorized, "missing or invalid bearer token", nil)
				return
			}
			meHandler.Me(w, r, userID)
		})))
	}

	mux.HandleFunc("/health", healthHandler.Check)
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.ListUsers(w, r)
		case http.MethodPost:
			userHandler.CreateUser(w, r)
		default:
			response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
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
	})
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

	chain := RequestID(
		Recovery(logger)(
			AccessLog(logger)(
				BodyLimit(opts.MaxBodyBytes)(mux),
			),
		),
	)
	return chain
}
