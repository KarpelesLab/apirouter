package apirouter

import "errors"

type Error struct {
	Message string // error message
	Code    int    // HTTP status code for error
	Token   string // optional error token
}

var (
	ErrTargetMissing = errors.New("missing target")
)

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) HTTPStatus() int {
	return e.Code
}
