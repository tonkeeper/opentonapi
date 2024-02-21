package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
)

var (
	queueLengthMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "streaming_api_queue_length",
		Help:    "Number of events sitting in a streaming connection's queue waiting to be sent.",
		Buckets: []float64{1, 10, 50, 100, 200, 300, 400, 500, 750, 1000, 1500, 2500, 5000, 7500, 10000},
	}, []string{"type", "event"})
)

func SseQueueLength(event events.Name, length int) {
	queueLengthMetric.With(map[string]string{"type": "sse", "event": event.String()}).Observe(float64(length))
}

func WebsocketQueueLength(event events.Name, length int) {
	queueLengthMetric.With(map[string]string{"type": "websocket", "event": event.String()}).Observe(float64(length))
}
