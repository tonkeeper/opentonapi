package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ogen-go/ogen/middleware"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"go.uber.org/zap"
	"net/http"
)

func ogenLoggingMiddleware(logger *zap.Logger) middleware.Middleware {
	return func(req middleware.Request, next middleware.Next) (middleware.Response, error) {
		logger := logger.With(
			zap.String("operation", req.OperationName),
			zap.String("path", req.Raw.URL.Path),
		)
		logger.Info("Handling request")
		resp, err := next(req)
		if err != nil {
			if oasError, ok := err.(*oas.ErrorStatusCode); ok && oasError.StatusCode == http.StatusInternalServerError {
				logger.Error("Fail", zap.Error(err))
			} else {
				logger.Info("Fail", zap.Error(err))
			}
		} else {
			logger.Info("Success")
		}
		return resp, err
	}
}

func asyncOperation(req *http.Request) string {
	return req.URL.Path
}

func asyncLoggingMiddleware(logger *zap.Logger) func(next AsyncHandler) AsyncHandler {
	return func(next AsyncHandler) AsyncHandler {
		return func(w http.ResponseWriter, r *http.Request, connectionType int, allowTokenInQuery bool) error {
			logger := logger.With(
				zap.String("operation", asyncOperation(r)),
				zap.String("path", r.URL.Path),
			)
			logger.Info("Handling request")
			if err := next(w, r, connectionType, allowTokenInQuery); err != nil {
				logger.Error("Fail", zap.Error(err))
				return err
			}
			logger.Info("Success")
			return nil
		}
	}
}

var httpResponseTimeMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Subsystem:   "http",
	Name:        "request_duration_seconds",
	Help:        "",
	ConstLabels: nil,
	Buckets:     []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 10},
}, []string{"operation"})

func ogenMetricsMiddleware(req middleware.Request, next middleware.Next) (middleware.Response, error) {
	t := prometheus.NewTimer(httpResponseTimeMetric.WithLabelValues(req.OperationName))
	defer t.ObserveDuration()
	return next(req)
}

func asyncMetricsMiddleware(next AsyncHandler) AsyncHandler {
	return func(w http.ResponseWriter, r *http.Request, connectionType int, allowTokenInQuery bool) error {
		t := prometheus.NewTimer(httpResponseTimeMetric.WithLabelValues(asyncOperation(r)))
		defer t.ObserveDuration()
		return next(w, r, connectionType, allowTokenInQuery)
	}
}

var ErrRateLimit = errors.New("rate limit")

type errorJSON struct {
	Error string
}

func ogenErrorsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("content-type", "application/json")
	switch err.(type) {
	case *ogenerrors.DecodeParamsError:
		// a quick workaround on Sunday
		// todo: fix it properly
		w.WriteHeader(http.StatusBadRequest)
	default:
		if errors.Is(err, ErrRateLimit) {
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	json.NewEncoder(w).Encode(&errorJSON{Error: err.Error()})
}
