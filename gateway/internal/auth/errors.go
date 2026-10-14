package auth

import (
	"marketplace/gateway/pkg/response"
	"net/http"
)

type ErrorResponse struct {
	Error   string       `json:"error"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

const (
	ErrKeyValidationFailed = "validation_failed"
	ErrKeyConflict         = "conflict"
	ErrKeyInternal         = "internal"
	ErrKeyUnauthenticated  = "unauthenticated"
)

func writeError(
	w http.ResponseWriter,
	status int,
	code string,
	message string,
) {
	response.Body(w, ErrorResponse{
		Error:   code,
		Message: message,
	}, status)
}

func writeConflictError(w http.ResponseWriter, message string) {
	response.Body(w, ErrorResponse{
		Error:   ErrKeyConflict,
		Message: message,
	}, http.StatusConflict)
}

func writeValidationError(w http.ResponseWriter, message string) {
	response.Body(w, ErrorResponse{
		Error:   ErrKeyValidationFailed,
		Message: message,
	}, http.StatusBadRequest)
}

func writeUnauthenticatedError(w http.ResponseWriter, message string) {
	response.Body(w, ErrorResponse{
		Error:   ErrKeyUnauthenticated,
		Message: message,
	}, http.StatusUnauthorized)
}

func writeInternalError(w http.ResponseWriter, message string) {
	response.Body(w, ErrorResponse{
		Error:   ErrKeyInternal,
		Message: message,
	}, http.StatusInternalServerError)
}
