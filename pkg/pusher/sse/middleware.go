package sse

import (
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/pusher/errors"
)

func writeError(writer http.ResponseWriter, err error) {
	if errors.IsHTTPError(err) {
		httpErr := err.(errors.HTTPError)
		writer.WriteHeader(httpErr.Code)
		writer.Write([]byte(httpErr.Message))
		return
	}
	writer.WriteHeader(http.StatusInternalServerError)
	writer.Write([]byte(err.Error()))
}

func Stream(handler HandlerFunc) func(writer http.ResponseWriter, request *http.Request) error {
	return func(writer http.ResponseWriter, request *http.Request) error {
		_, ok := writer.(http.Flusher)
		if !ok {
			err := errors.InternalServerError("streaming unsupported")
			writeError(writer, err)
			return err
		}

		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")
		writer.Header().Set("Transfer-Encoding", "chunked")

		// TODO: last-event-id
		session := newSession()
		if err := handler(session, request); err != nil {
			writeError(writer, err)
			return err
		}
		if err := session.StreamEvents(request.Context(), writer); err != nil {
			writeError(writer, err)
			return err
		}
		return nil
	}
}
