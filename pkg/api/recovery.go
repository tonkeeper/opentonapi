package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var httpPanicsCounter = promauto.NewCounter(prometheus.CounterOpts{
	Subsystem: "http",
	Name:      "handler_panics_total",
	Help:      "Number of panics recovered in HTTP handlers",
})

// recoveryMiddleware catches a panic raised while serving a request, logs it and responds with HTTP 500
// so that a single broken request doesn't abort the connection with an empty reply (which a load balancer
// reports as 502).
//
// It only catches panics in the request goroutine.
// A panic in a goroutine spawned by a handler still crashes the process,
// so handlers must run such goroutines with conc.WaitGroup, which re-panics in the caller's goroutine on Wait().
func recoveryMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &recoveryResponseWriter{ResponseWriter: w}
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				// http.ErrAbortHandler is the documented way to abort a response, net/http handles it silently.
				panic(rec)
			}
			httpPanicsCounter.Inc()
			logger.Error("panic in http handler",
				zap.String("path", r.URL.Path),
				zap.String("panic", fmt.Sprint(rec)),
				zap.ByteString("stack", debug.Stack()))
			if rw.wroteHeader {
				// The response is partially sent, we can't replace it with an error.
				// Abort the connection so the client doesn't take a truncated response as a complete one.
				panic(http.ErrAbortHandler)
			}
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(rw).Encode(&errorJSON{Error: "internal server error"})
		}()
		next.ServeHTTP(rw, r)
	})
}

type recoveryResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *recoveryResponseWriter) WriteHeader(statusCode int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *recoveryResponseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

// Flush keeps streaming endpoints (SSE, GraphQL stream) working through the wrapper.
func (w *recoveryResponseWriter) Flush() {
	w.wroteHeader = true
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *recoveryResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
