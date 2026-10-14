package auth

import (
	"log"
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
	if err := response.Body(w, ErrorResponse{
		Error:   code,
		Message: message,
	}, status); err != nil {
		log.Printf("write error response: %v", err)
	}
}

func writeConflictError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusConflict, ErrKeyConflict, message)
}

func writeValidationError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, ErrKeyValidationFailed, message)
}

func writeUnauthenticatedError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusUnauthorized, ErrKeyUnauthenticated, message)
}

func writeInternalError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusInternalServerError, ErrKeyInternal, message)
}
