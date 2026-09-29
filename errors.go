package ogmake

import (
	"encoding/json"
	"fmt"
)

// ErrorDetail is one field-level validation message.
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError is returned when the API answers a non-2xx status. Status 0 with
// Code "unauthenticated" means no API key was configured (no request was sent).
type APIError struct {
	Code   string
	Status int
	Detail []ErrorDetail
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ogmake API error: %s (HTTP %d)", e.Code, e.Status)
}

// NetworkError wraps a transport-level failure.
type NetworkError struct{ Err error }

func (e *NetworkError) Error() string { return "ogmake network error: " + e.Err.Error() }
func (e *NetworkError) Unwrap() error { return e.Err }

// RedirectError is returned for any 3xx; the SDK never follows redirects.
type RedirectError struct {
	Status   int
	Location string
}

func (e *RedirectError) Error() string {
	return fmt.Sprintf("ogmake API returned a redirect (HTTP %d); the SDK never follows redirects", e.Status)
}

func toAPIError(body []byte, status int) *APIError {
	var parsed struct {
		Error  string        `json:"error"`
		Detail []ErrorDetail `json:"detail"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Error == "" {
		return &APIError{Code: "unknown_error", Status: status}
	}
	var detail []ErrorDetail
	for _, d := range parsed.Detail {
		if d.Field != "" && d.Message != "" {
			detail = append(detail, d)
		}
	}
	return &APIError{Code: parsed.Error, Status: status, Detail: detail}
}
