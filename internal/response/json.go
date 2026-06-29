package response

import (
	"encoding/json"
	"net/http"

	"go-lang/internal/model"
)

func JSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}

func Error(w http.ResponseWriter, statusCode int, code, message string, details map[string][]string) {
	JSON(w, statusCode, model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func BadRequest(w http.ResponseWriter, code, message string, details map[string][]string) {
	Error(w, http.StatusBadRequest, code, message, details)
}

func NotFound(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusNotFound, code, message, nil)
}

func Conflict(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusConflict, code, message, nil)
}

func MethodNotAllowed(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusMethodNotAllowed, code, message, nil)
}

func UnsupportedMediaType(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusUnsupportedMediaType, code, message, nil)
}

func InternalError(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusInternalServerError, code, message, nil)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
