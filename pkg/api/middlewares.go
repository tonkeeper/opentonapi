package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ogen-go/ogen/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"net/http"
)

func Logging(logger *zap.Logger) middleware.Middleware {
	return func(
		req middleware.Request,
		next func(req middleware.Request) (middleware.Response, error),
	) (middleware.Response, error) {
		logger := logger.With(
			zap.String("operation", req.OperationName),
			zap.String("path", req.Raw.URL.Path),
		)
		logger.Info("Handling request")
		resp, err := next(req)
		if err != nil {
			logger.Error("Fail", zap.Error(err))
		} else {
			var fields []zap.Field
			if tresp, ok := resp.Type.(interface{ GetStatusCode() int }); ok {
				fields = []zap.Field{
					zap.Int("status_code", tresp.GetStatusCode()),
				}
			}
			logger.Info("Success", fields...)
		}
		return resp, err
	}
}

var httpResponseTimeMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Subsystem:   "http",
	Name:        "request_duration_seconds",
	Help:        "",
	ConstLabels: nil,
	Buckets:     []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 10},
}, []string{"operation"})

func Metrics(req middleware.Request,
	next func(req middleware.Request) (middleware.Response, error),
) (middleware.Response, error) {
	t := prometheus.NewTimer(httpResponseTimeMetric.WithLabelValues(req.OperationName))
	defer t.ObserveDuration()
	return next(req)
}

var ErrRateLimit = errors.New("rate limit")

func ErrorsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("content-type", "application/json")
	text := err.Error()
	if errors.Is(err, ErrRateLimit) {
		w.WriteHeader(http.StatusTooManyRequests)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: text})
}
