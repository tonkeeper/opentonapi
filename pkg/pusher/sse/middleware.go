package sse

import (
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/pusher/errors"
	"github.com/tonkeeper/opentonapi/pkg/pusher/metrics"
	"github.com/tonkeeper/opentonapi/pkg/pusher/utils"
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

func Stream(handler handlerFunc) func(http.ResponseWriter, *http.Request, int, bool) error {
	return func(writer http.ResponseWriter, request *http.Request, connectionType int, allowTokenInQuery bool) error {
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

		metrics.OpenSseConnection(utils.TokenNameFromContext(request.Context()))
		defer metrics.CloseSseConnection(utils.TokenNameFromContext(request.Context()))

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
