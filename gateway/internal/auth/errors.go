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