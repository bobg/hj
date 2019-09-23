package hj

import (
	"fmt"
	"net/http"
)

// ErrNotPost is the type of error produced when a handler created by Handler
// is invoked with an HTTP method other than POST.
type ErrNotPost struct {
	// Method is the HTTP method used to invoke the handler.
	Method string
}

func (e ErrNotPost) Error() string {
	return fmt.Sprintf("HTTP method is %s but must be POST", e.Method)
}

func (e ErrNotPost) As(target interface{}) bool {
	if c, ok := target.(*CodeErr); ok {
		*c = CodeErr{C: http.StatusBadRequest}
		return true
	}
	return false
}

// ErrNotJSON is the type of error produced when a handler created by Handler
// is invoked with a request body not properly labeled as JSON.
type ErrNotJSON struct{
	// ContentType is the content type of the request body.
	ContentType string
}

func (e ErrNotJSON) Error() string {
	return fmt.Sprintf("request Content-Type is %s, want application/json", e.ContentType)
}

func (e ErrNotJSON) As(target interface{}) bool {
	if c, ok := target.(*CodeErr); ok {
		*c = CodeErr{C: http.StatusBadRequest}
		return true
	}
	return false
}

// ErrDecode is the type of error produced when a handler created by Handler
// cannot JSON-decode the request body.
type ErrDecode struct {
	// Err is the error from the JSON package.
	Err error
}

func (e ErrDecode) Error() string {
	return "while decoding JSON request body: " + e.Err.Error()
}

func (e ErrDecode) Unwrap() error {
	return e.Err
}

func (e ErrDecode) As(target interface{}) bool {
	if c, ok := target.(*CodeErr); ok {
		*c = CodeErr{C: http.StatusBadRequest}
		return true
	}
	return false
}

// ErrEncode is the type of error produced when a handler created by Handler
// cannot JSON-encode the response
type ErrEncode struct {
	// Err is the error from the JSON package.
	Err error
}

func (e ErrEncode) Error() string {
	return "while encoding JSON response: " + e.Err.Error()
}

func (e ErrEncode) Unwrap() error {
	return e.Err
}

func (e ErrEncode) As(target interface{}) bool {
	if c, ok := target.(*CodeErr); ok {
		*c = CodeErr{C: http.StatusBadRequest}
		return true
	}
	return false
}

// CodeErr is an error that can be returned from the function wrapped by hj.Handler
// to control the HTTP status code returned from the pending request.
type CodeErr struct {
	// C is an HTTP status code.
	C int

	// Err is an optional wrapped error.
	Err error
}

func (c CodeErr) Error() string {
	s := fmt.Sprintf("HTTP %d", c.C)
	if t := http.StatusText(c.C); t != "" {
		s += ": " + t
	}
	if c.Err != nil {
		s += ": " + c.Err.Error()
	}
	return s
}

func (c CodeErr) Unwrap() error {
	return c.Err
}

func (c CodeErr) Respond(w http.ResponseWriter) {
	http.Error(w, c.Error(), c.C)
}

type Responder interface {
	Respond(http.ResponseWriter)
}
