package apirouter

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
)

type Error struct {
	Message string // error message
	Code    int    // HTTP status code for error
	Token   string // optional error token
	parent  error  // for unwrap
}

var (
	ErrTargetMissing         = errors.New("missing target")
	ErrNotFound              = &Error{Message: "Not found", Token: "error_not_found", Code: http.StatusNotFound, parent: fs.ErrNotExist}
	ErrAccessDenied          = &Error{Message: "Access denied", Token: "error_access_denied", Code: http.StatusForbidden}
	ErrInternal              = &Error{Message: "An internal error occured", Token: "error_internal", Code: http.StatusInternalServerError}
	ErrInsecureRequest       = &Error{Message: "Request must use POST and have the appropriate tokens", Token: "error_insecure_request", Code: http.StatusBadRequest}
	ErrTeapot                = &Error{Message: "A teapot has appeared", Token: "error_teapot", Code: http.StatusTeapot}
	ErrLengthRequired        = &Error{Message: "Content-Length header is required for this request", Token: "error_length_required", Code: http.StatusLengthRequired}
	ErrRequestEntityTooLarge = &Error{Message: "Request body is too large", Token: "error_request_entity_too_large", Code: http.StatusRequestEntityTooLarge}
)

func NewError(code int, token, msg string, args ...any) *Error {
	err := fmt.Errorf(msg, args...)
	return &Error{
		Message: err.Error(),
		Code:    code,
		Token:   token,
		parent:  errors.Unwrap(err),
	}
}

// ErrBadRequest is a helper returning an error with code StatusBadRequest
func ErrBadRequest(token, msg string, args ...any) *Error {
	return NewError(http.StatusBadRequest, token, msg, args...)
}

// Forbidden is a helper returning an error with code StatusForbidden
func ErrForbidden(token, msg string, args ...any) *Error {
	return NewError(http.StatusForbidden, token, msg, args...)
}

func ErrMethodNotAllowed(token, msg string, args ...any) *Error {
	return NewError(http.StatusMethodNotAllowed, token, msg, args...)
}

func ErrInternalServerError(token, msg string, args ...any) *Error {
	return NewError(http.StatusInternalServerError, token, msg, args...)
}

func ErrNotImplemented(token, msg string, args ...any) *Error {
	return NewError(http.StatusNotImplemented, token, msg, args...)
}

func ErrServiceUnavailable(token, msg string, args ...any) *Error {
	return NewError(http.StatusServiceUnavailable, token, msg, args...)
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) HTTPStatus() int {
	return e.Code
}

func (e *Error) Unwrap() error {
	return e.parent
}
