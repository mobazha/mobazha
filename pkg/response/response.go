package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
)

type SuccessEnvelope struct {
	Data interface{} `json:"data"`
	Meta *Meta       `json:"meta,omitempty"`
}

type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

type Meta struct {
	Page       int    `json:"page,omitempty"`
	PageSize   int    `json:"pageSize,omitempty"`
	Total      int64  `json:"total,omitempty"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type APIError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Detail  string       `json:"detail,omitempty"`
	TraceID string       `json:"traceID,omitempty"`
	Details []FieldError `json:"details,omitempty"`
	// Data carries structured domain-specific payload for callers to act on
	// (e.g. conflict resolution metadata). Use sparingly; field-level errors
	// should still go through Details.
	Data interface{} `json:"data,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

const (
	CodeValidation      = "VALIDATION_ERROR"
	CodeBadRequest      = "BAD_REQUEST"
	CodeNotFound        = "NOT_FOUND"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeConflict        = "CONFLICT"
	CodeInternalError   = "INTERNAL_ERROR"
	CodeNotImplemented  = "NOT_IMPLEMENTED"
	CodeServiceUnavail  = "SERVICE_UNAVAILABLE"
	CodePayloadTooLarge = "PAYLOAD_TOO_LARGE"
	CodeProviderError   = "PROVIDER_ERROR"
	CodeRateLimited     = "RATE_LIMITED"
	// CodeAccountLinkConflict signals that an OAuth provider ID (e.g. Telegram,
	// Discord) is already bound to a different Casdoor account. The response
	// should include APIError.Data with the conflicting account's metadata so
	// the client can guide the user to a resolution path (e.g. account merge).
	CodeAccountLinkConflict = "ACCOUNT_LINK_CONFLICT"
)

// Success writes 200 + {"data": T}.
func Success(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, SuccessEnvelope{Data: data})
}

// Created writes 201 + {"data": T}.
func Created(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusCreated, SuccessEnvelope{Data: data})
}

// StatusWithData writes status + {"data": T}. Used for success-like responses with non-200/201 status (e.g. 409 Conflict).
func StatusWithData(w http.ResponseWriter, status int, data interface{}) {
	writeJSON(w, status, SuccessEnvelope{Data: data})
}

// List writes 200 + {"data": T[], "meta": {...}}.
func List(w http.ResponseWriter, data interface{}, meta Meta) {
	writeJSON(w, http.StatusOK, SuccessEnvelope{Data: data, Meta: &meta})
}

// NoContent writes 204 with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error writes {"error": {"code": code, "message": message}}.
func Error(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorEnvelope{
		Error: APIError{
			Code:    code,
			Message: message,
		},
	})
}

// ErrorWithDetail writes a structured error with a technical detail summary
// and trace ID for diagnostics. The detail field should contain a sanitized
// technical description (never include API keys, tokens, or secrets).
// The traceID is automatically extracted from the X-Request-ID response header.
func ErrorWithDetail(w http.ResponseWriter, status int, code, message, detail string) {
	traceID := w.Header().Get("X-Request-ID")
	writeJSON(w, status, ErrorEnvelope{
		Error: APIError{
			Code:    code,
			Message: message,
			Detail:  detail,
			TraceID: traceID,
		},
	})
}

// ErrorWithData writes a structured error with a domain-specific payload so
// the client can act on the error (e.g. conflict resolution). Keep the payload
// free of secrets and PII that the caller isn't already authorized to see.
func ErrorWithData(w http.ResponseWriter, status int, code, message string, data interface{}) {
	traceID := w.Header().Get("X-Request-ID")
	writeJSON(w, status, ErrorEnvelope{
		Error: APIError{
			Code:    code,
			Message: message,
			TraceID: traceID,
			Data:    data,
		},
	})
}

// ErrorValidation writes 400 + field-level validation errors.
func ErrorValidation(w http.ResponseWriter, details []FieldError) {
	writeJSON(w, http.StatusBadRequest, ErrorEnvelope{
		Error: APIError{
			Code:    CodeValidation,
			Message: "Invalid input",
			Details: details,
		},
	})
}

// HttpStatusToCode maps an HTTP status code to a standard error code string.
func HttpStatusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusNotImplemented:
		return CodeNotImplemented
	case http.StatusRequestEntityTooLarge:
		return CodePayloadTooLarge
	case http.StatusServiceUnavailable:
		return CodeServiceUnavail
	default:
		return CodeInternalError
	}
}

// PanicRecovery recovers from panics and returns a 500 JSON error.
// Register after RequestID middleware so the response header X-Request-ID
// is already set and available for logging.
func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				reqID := w.Header().Get("X-Request-ID")
				fmt.Printf("[PANIC] request_id=%s method=%s path=%s err=%v\n%s\n",
					reqID, r.Method, r.URL.Path, err, debug.Stack())
				ErrorWithDetail(w, http.StatusInternalServerError, CodeInternalError,
					"An unexpected error occurred", fmt.Sprintf("panic: %v", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Printf("[response] JSON encode error: %v\n", err)
	}
}
