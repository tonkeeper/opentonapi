package sse

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
)

// Session represents an HTTP connection from a client and
// implements a loop to stream events from a channel to http.ResponseWriter.
type Session struct {
	eventCh      chan Event
	cancel       sources.CancelFn
	pingInterval time.Duration
}

func newSession() *Session {
	return &Session{
		// TODO: use elastic channel to be sure transactionDispatcher doesn't hang
		eventCh:      make(chan Event, 100),
		pingInterval: 5 * time.Second,
	}
}

func (s *Session) SendEvent(event Event) {
	s.eventCh <- event
}

func (s *Session) SetCancelFn(cancel sources.CancelFn) {
	s.cancel = cancel
}

func (s *Session) StreamEvents(ctx context.Context, writer http.ResponseWriter) error {
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
			_, err = fmt.Fprintf(writer, "id: %v\ndata: %v\n\n", msg.EventID, string(msg.Data))
		case <-time.After(s.pingInterval):
			_, err = fmt.Fprintf(writer, "body: heartbeat\n\n")
		}
		if err != nil {
			// closing a connection
			return err
		}
		flusher.Flush()
	}
}
