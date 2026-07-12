// Package httpresponse defines the public HTTP response contract shared by all modules.
package httpresponse

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Data  any       `json:"data,omitempty"`
	Error *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
	Details   any    `json:"details,omitempty"`
}

func Success(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, envelope{Data: data})
}

func Failure(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, envelope{Error: &APIError{
		Code: code, Message: message, Details: details,
	}})
}

func writeJSON(w http.ResponseWriter, status int, value envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
