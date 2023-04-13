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

func BadRequest(msg string) HTTPError {
	return HTTPError{
		Code:    http.StatusBadRequest,
		Message: msg,
	}
}
