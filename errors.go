package apirouter

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
)

// Error represents a structured API error with an HTTP status code and optional token.
// The Token field can be used by clients for programmatic error handling,
// while Message provides a human-readable description.
type Error struct {
	Message string // error message
	Code    int    // HTTP status code for error
	Token   string // optional error token for programmatic handling
	Info    any    // optional extra information for the error
	parent  error  // for unwrap
}

// Common error values that can be returned from API handlers.
var (
	// ErrTargetMissing indicates a required target URL is missing.
	ErrTargetMissing = errors.New("missing target")

	// ErrNotFound indicates the requested resource was not found (404).
	ErrNotFound = &Error{Message: "Not found", Token: "error_not_found", Code: http.StatusNotFound, parent: fs.ErrNotExist}

	// ErrAccessDenied indicates the request was forbidden (403).
	ErrAccessDenied = &Error{Message: "Access denied", Token: "error_access_denied", Code: http.StatusForbidden}

	// ErrInternal indicates an internal server error occurred (500).
	ErrInternal = &Error{Message: "An internal error occurred", Token: "error_internal", Code: http.StatusInternalServerError}

	// ErrInsecureRequest indicates the request lacks required security tokens (400).
	ErrInsecureRequest = &Error{Message: "Request must use POST and have the appropriate tokens", Token: "error_insecure_request", Code: http.StatusBadRequest}

	// ErrTeapot is an easter egg error (418).
	ErrTeapot = &Error{Message: "A teapot has appeared", Token: "error_teapot", Code: http.StatusTeapot}

	// ErrLengthRequired indicates Content-Length header is required (411).
	ErrLengthRequired = &Error{Message: "Content-Length header is required for this request", Token: "error_length_required", Code: http.StatusLengthRequired}

	// ErrRequestEntityTooLarge indicates the request body exceeds size limits (413).
	ErrRequestEntityTooLarge = &Error{Message: "Request body is too large", Token: "error_request_entity_too_large", Code: http.StatusRequestEntityTooLarge}
)

// NewError creates a new Error with the specified HTTP status code, token, and formatted message.
// The msg parameter supports fmt.Errorf style formatting with args.
func NewError(code int, token, msg string, args ...any) *Error {
	err := fmt.Errorf(msg, args...)
	return &Error{
		Message: err.Error(),
		Code:    code,
		Token:   token,
		parent:  errors.Unwrap(err),
	}
}

// ErrBadRequest creates an error with HTTP status 400 Bad Request.
func ErrBadRequest(token, msg string, args ...any) *Error {
	return NewError(http.StatusBadRequest, token, msg, args...)
}

// ErrForbidden creates an error with HTTP status 403 Forbidden.
func ErrForbidden(token, msg string, args ...any) *Error {
	return NewError(http.StatusForbidden, token, msg, args...)
}

// ErrMethodNotAllowed creates an error with HTTP status 405 Method Not Allowed.
func ErrMethodNotAllowed(token, msg string, args ...any) *Error {
	return NewError(http.StatusMethodNotAllowed, token, msg, args...)
}

// ErrInternalServerError creates an error with HTTP status 500 Internal Server Error.
func ErrInternalServerError(token, msg string, args ...any) *Error {
	return NewError(http.StatusInternalServerError, token, msg, args...)
}

// ErrNotImplemented creates an error with HTTP status 501 Not Implemented.
func ErrNotImplemented(token, msg string, args ...any) *Error {
	return NewError(http.StatusNotImplemented, token, msg, args...)
}

// ErrServiceUnavailable creates an error with HTTP status 503 Service Unavailable.
func ErrServiceUnavailable(token, msg string, args ...any) *Error {
	return NewError(http.StatusServiceUnavailable, token, msg, args...)
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// HTTPStatus returns the HTTP status code for this error.
func (e *Error) HTTPStatus() int {
	return e.Code
}

// Unwrap returns the underlying error, if any, for use with errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.parent
}
