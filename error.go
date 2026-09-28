package ticketbai

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invopop/gobl.ticketbai/internal/gateways"
)

// Main error types return by this package.
var (
	// ErrValidation implies there is something wrong with the contents of the
	// document that needs to be fixed before sending it again.
	ErrValidation = newError("validation")
	// ErrDuplicate is used when the gateway has already received the document.
	ErrDuplicate = newError("duplicate")
	// ErrConnection is used when the gateway could not be reached or gave a
	// response we're unable to understand.
	ErrConnection = newError("connection")
	// ErrServer indicates the gateway had an internal problem handling the
	// request, which may succeed if attempted again later.
	ErrServer = newError("server")
	// ErrInternal is used for any other unexpected error.
	ErrInternal = newError("internal")
)

// Error allows for structured responses to better handle errors upstream.
type Error struct {
	key     string
	code    string
	message string
	cause   error
}

func newError(key string) *Error {
	return &Error{key: key}
}

// newErrorFrom attempts to wrap the provided error into the Error type. Errors
// from the gateways are searched for in the chain so that their key and code
// are preserved even if they were wrapped with additional context.
func newErrorFrom(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	var ge *gateways.Error
	if errors.As(err, &ge) {
		return &Error{
			key:     ge.Key(),
			code:    ge.Code(),
			message: ge.Message(),
			cause:   err,
		}
	}
	return &Error{
		key:     "internal",
		message: err.Error(),
		cause:   err,
	}
}

// Error produces a human readable error message.
func (e *Error) Error() string {
	out := []string{e.key}
	if e.code != "" {
		out = append(out, e.code)
	}
	if e.message != "" {
		out = append(out, e.message)
	}
	return strings.Join(out, ": ")
}

// Key returns the key for the error.
func (e *Error) Key() string {
	return e.key
}

// Message returns the human message for the error.
func (e *Error) Message() string {
	return e.message
}

// Code returns the code provided by the remote service.
func (e *Error) Code() string {
	return e.code
}

// Cause returns the undlying error that caused this error.
func (e *Error) Cause() error {
	return e.cause
}

// withMessage duplicates and adds the message to the error.
func (e *Error) withMessage(msg string, args ...any) *Error {
	e = e.clone()
	e.message = fmt.Sprintf(msg, args...)
	return e
}

// withCause duplicates and adds the cause to the error.
func (e *Error) withCause(err error) *Error {
	e = e.clone()
	if e.message == "" {
		e.message = err.Error()
	}
	e.cause = err
	return e
}

func (e *Error) clone() *Error {
	ne := new(Error)
	*ne = *e
	return ne
}

// Is checks to see if the target error is the same as the current one
// or forms part of the chain.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return errors.Is(e.cause, target)
	}
	return e.key == t.key
}
