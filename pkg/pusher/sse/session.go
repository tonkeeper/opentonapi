package sse

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
	"github.com/tonkeeper/opentonapi/pkg/pusher/metrics"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
	"github.com/tonkeeper/opentonapi/pkg/pusher/utils"
	"go.uber.org/zap"
)

// session represents an HTTP connection from a client and
// implements a loop to stream events from a channel to http.ResponseWriter.
type session struct {
	logger       *zap.Logger
	eventCh      chan Event
	cancel       sources.CancelFn
	pingInterval time.Duration
}

func newSession(logger *zap.Logger) *session {
	return &session{
		logger:       logger,
		eventCh:      make(chan Event, 1000),
		pingInterval: 5 * time.Second,
	}
}

func (s *session) SendEvent(event Event) {
	select {
	case s.eventCh <- event:
	default:
		// TODO: maybe we should either close the channel or let the user know that we have dropped an event
		s.logger.Warn("event channel is full, dropping event",
			zap.String("event", string(event.Name)))
	}
}

func (s *session) SetCancelFn(cancel sources.CancelFn) {
	s.cancel = cancel
}

func (s *session) StreamEvents(ctx context.Context, writer http.ResponseWriter) error {
	defer s.cancel()

	flusher := writer.(http.Flusher)
	for {
		var err error
		select {
		case <-ctx.Done():
			return nil
		case msg, open := <-s.eventCh:
			if !open {
				return nil
			}
			_, err = fmt.Fprintf(writer, "event: message\nid: %v\ndata: %v\n\n", msg.EventID, string(msg.Data))
			metrics.SseEventSent(msg.Name, utils.TokenNameFromContext(ctx))
		case <-time.After(s.pingInterval):
			metrics.SseEventSent(events.PingEvent, utils.TokenNameFromContext(ctx))
			_, err = fmt.Fprintf(writer, "event: heartbeat\n\n")
		}
		if err != nil {
			// closing a connection
			return err
		}
		flusher.Flush()
	}
}
