package server

import (
	"log/slog"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	"go-lang/internal/auth"
	"go-lang/internal/handler"
	"go-lang/internal/model"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

type Options struct {
	MaxBodyBytes int64
	TokenIssuer  *auth.TokenIssuer
	BcryptCost   int
}

func New(userStore store.UserStore, logger *slog.Logger, opts Options) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	healthHandler := handler.NewHealthHandler()
	userHandler := handler.NewUserHandler(userStore, opts.BcryptCost)
	meHandler := handler.NewMeHandler(userStore)

	var authHandler handler.AuthHandler
	if opts.TokenIssuer != nil {
		authHandler = handler.NewAuthHandler(userStore, opts.TokenIssuer)
		mux.HandleFunc("/auth/login", authHandler.Login)
		mux.Handle("/me", RequireAuth(opts.TokenIssuer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
