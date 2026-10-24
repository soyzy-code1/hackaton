package apperr

import "net/http"

type AppError struct {
	Status  int
	Message string
	Code    string
}

func (e *AppError) Error() string { return e.Message }

func New(status int, message, code string) *AppError {
	return &AppError{Status: status, Message: message, Code: code}
}

var (
	AuthRequired    = New(http.StatusUnauthorized, "Authentication required", "AUTH_REQUIRED")
	Forbidden       = New(http.StatusForbidden, "Insufficient permissions", "FORBIDDEN")
	NotFound        = New(http.StatusNotFound, "Not found", "NOT_FOUND")
	ValidationError = New(http.StatusUnprocessableEntity, "Validation error", "VALIDATION_ERROR")
	Conflict        = New(http.StatusConflict, "Conflict", "CONFLICT")
)
