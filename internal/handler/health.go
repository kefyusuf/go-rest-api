package handler

import (
	"net/http"

	"go-lang/internal/model"
	"go-lang/internal/response"
)

type HealthHandler struct{}

func NewHealthHandler() HealthHandler {
	return HealthHandler{}
}

// Check godoc
// @Summary Health check
// @Description Confirms that the application is running
// @Tags health
// @Produce json
// @Success 200 {object} model.HealthResponse
// @Failure 405 {object} model.ErrorResponse
// @Router /health [get]
func (h HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}

	response.JSON(w, http.StatusOK, model.HealthResponse{
		Status:  "ok",
		Message: "API is running",
	})
}
