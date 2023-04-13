package sse

import (
	"fmt"
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

func StreamingMiddleware(handler HandlerFunc) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, ok := writer.(http.Flusher)
		if !ok {
			writeError(writer, fmt.Errorf("streaming unsupported"))
			return
		}

		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")
		writer.Header().Set("Transfer-Encoding", "chunked")

		// TODO: last-event-id
		session := newSession()
		if err := handler(session, request); err != nil {
			writeError(writer, err)
			return
		}
		if err := session.StreamEvents(request.Context(), writer); err != nil {
			writeError(writer, err)
			return
		}
	})
}
