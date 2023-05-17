package sse

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
	"github.com/tonkeeper/opentonapi/pkg/pusher/metrics"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
)

// session represents an HTTP connection from a client and
// implements a loop to stream events from a channel to http.ResponseWriter.
type session struct {
	eventCh      chan Event
	cancel       sources.CancelFn
	pingInterval time.Duration
}

func newSession() *session {
	return &session{
		// TODO: use elastic channel to be sure transactionDispatcher doesn't hang
		eventCh:      make(chan Event, 100),
		pingInterval: 5 * time.Second,
	}
}

func (s *session) SendEvent(event Event) {
	s.eventCh <- event
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
			metrics.SseEventSent(msg.Name)
		case <-time.After(s.pingInterval):
			metrics.SseEventSent(events.PingEvent)
			_, err = fmt.Fprintf(writer, "event: heartbeat\n\n")
		}
		if err != nil {
			// closing a connection
			return err
		}
		flusher.Flush()
	}
}
