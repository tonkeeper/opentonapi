package sse

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_session_Stream(t *testing.T) {
	// to make "go test -race" happy
	cancelIsCalled := atomic.Bool{}
	s := &session{
		eventCh: make(chan Event, 10),
		cancel: func() {
			cancelIsCalled.Store(true)
		},
		pingInterval: time.Second * 1,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	rec := httptest.NewRecorder()
	go func() {
		err := s.StreamEvents(ctx, rec)
		require.Nil(t, err)
	}()
	go func() {
		s.eventCh <- Event{EventID: 1, Data: []byte("hello")}
		s.eventCh <- Event{EventID: 2, Data: []byte("hello")}
	}()
	select {
	case <-ctx.Done():
	}
	time.Sleep(1 * time.Second)
	require.True(t, cancelIsCalled.Load())
	expectedBody := `event: heartbeat

event: message
id: 1
data: hello

event: message
id: 2
data: hello

event: heartbeat

`
	require.Equal(t, expectedBody, rec.Body.String())
}

func Test_session_SendEvent(t *testing.T) {
	tests := []struct {
		name           string
		sendEventCount int
		wantEvents     map[int64]struct{}
	}{
		{
			name:           "some events dropped",
			sendEventCount: 10,
			wantEvents: map[int64]struct{}{
				0: {},
				1: {},
				2: {},
				3: {},
			},
		},
		{
			name:           "all events delivered",
			sendEventCount: 3,
			wantEvents: map[int64]struct{}{
				0: {},
				1: {},
				2: {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &session{
				logger:  zap.L(),
				eventCh: make(chan Event, 4),
			}
			for i := 0; i < tt.sendEventCount; i++ {
				s.SendEvent(Event{EventID: int64(i)})
			}

			close(s.eventCh)
			events := make(map[int64]struct{})
			for event := range s.eventCh {
				events[event.EventID] = struct{}{}
			}
			require.Equal(t, tt.wantEvents, events)
		})
	}
}
