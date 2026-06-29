package model

const (
	ErrorCodeBadRequest           = "BAD_REQUEST"
	ErrorCodeValidation           = "VALIDATION_ERROR"
	ErrorCodeNotFound             = "NOT_FOUND"
	ErrorCodeConflict             = "CONFLICT"
	ErrorCodeInternal             = "INTERNAL_ERROR"
	ErrorCodeMethodNotAllowed     = "METHOD_NOT_ALLOWED"
	ErrorCodeUnsupportedMediaType = "UNSUPPORTED_MEDIA_TYPE"
	ErrorCodeUnauthorized         = "UNAUTHORIZED"
)

type ErrorDetail struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Details map[string][]string `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
