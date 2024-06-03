package errors

import (
	"errors"
	"net/http"
)

type HTTPError struct {
	Code    int    `json:"-"`
	Message string `json:"error"`
}

func IsHTTPError(err error) bool {
	return errors.Is(err, HTTPError{})
}

func (e HTTPError) Error() string {
	return e.Message
}

func (e HTTPError) Is(err error) bool {
	_, ok := err.(HTTPError)
	return ok
}

func InternalServerError(msg string) HTTPError {
	return HTTPError{
		Code:    http.StatusInternalServerError,
		Message: msg,
	}
}
func BadRequest(msg string) HTTPError {
	return HTTPError{
		Code:    http.StatusBadRequest,
		Message: msg,
	}
}

func NotImplemented() HTTPError {
	return HTTPError{
		Code:    http.StatusNotImplemented,
		Message: "method not implemented",
	}
}
