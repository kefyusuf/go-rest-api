package server

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	"go-lang/internal/handler"
	"go-lang/internal/model"
	"go-lang/internal/response"
	"go-lang/internal/store"
)

func New(userStore store.UserStore) http.Handler {
	mux := http.NewServeMux()
	healthHandler := handler.NewHealthHandler()
	userHandler := handler.NewUserHandler(userStore)

	mux.HandleFunc("/health", healthHandler.Check)
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.ListUsers(w, r)
		case http.MethodPost:
			userHandler.CreateUser(w, r)
		default:
			response.MethodNotAllowed(w, []string{http.MethodGet, http.MethodPost}, model.ErrorCodeMethodNotAllowed, "method not allowed")
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
			response.MethodNotAllowed(w, []string{http.MethodGet, http.MethodPut, http.MethodDelete}, model.ErrorCodeMethodNotAllowed, "method not allowed")
		}
	})
	mux.Handle("/swagger/", httpSwagger.WrapHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{
			"message": "Welcome to the Go API starter",
		})
	})

	return Logging(mux)
}
