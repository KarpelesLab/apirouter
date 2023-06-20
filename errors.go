package apirouter

import (
	"errors"
	"net/http"
)

type Error struct {
	Message string // error message
	Code    int    // HTTP status code for error
	Token   string // optional error token
}

var (
	ErrTargetMissing   = errors.New("missing target")
	ErrAccessDenied    = &Error{Message: "Access denied", Token: "error_access_denied", Code: http.StatusForbidden}
	ErrInternal        = &Error{Message: "An internal error occured", Token: "error_internal", Code: http.StatusInternalServerError}
	ErrInsecureRequest = &Error{Message: "Request must use POST and have the appropriate tokens", Token: "error_insecure_request", Code: http.StatusBadRequest}
	ErrTeapot          = &Error{Message: "A teapot has appeared", Token: "error_teapot", Code: http.StatusTeapot}
)

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) HTTPStatus() int {
	return e.Code
}
