package sse

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	expectedBody := `event: message
id: 1
data: hello

event: message
id: 2
data: hello

event: heartbeat

`
	require.Equal(t, expectedBody, rec.Body.String())
}
