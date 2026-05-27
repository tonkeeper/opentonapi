package litestorage

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/tongo/liteclient"
)

var (
	liteServerRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "liteserver_request_duration_seconds",
			Help:    "Duration of lite server requests in seconds.",
			Buckets: []float64{0.001, 0.01, 0.05, 0.1, 1, 5, 10},
		},
		[]string{"host", "method", "status"},
	)
)

type LiteclientObserver struct{}

func (LiteclientObserver) ObserveRequest(host string, method liteclient.RequestName, duration time.Duration, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	liteServerRequestDuration.WithLabelValues(host, method, status).Observe(duration.Seconds())
}
