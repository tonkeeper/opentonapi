package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
)

var (
	eventsQuantity = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "streaming_api_events"},
		[]string{"type", "event", "token_name"},
	)

	droppedEvents = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "streaming_api_dropped_events",
		Help:    "Percent of dropped events per connection",
		Buckets: []float64{0, 1, 5, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
	}, []string{"type", "event"})
)

func SseEventSent(event events.Name, token string) {
	eventsQuantity.With(map[string]string{"type": "sse", "event": event.String(), "token_name": token}).Inc()
}

func SseDroppedEvents(event events.Name, percent float64) {
	droppedEvents.With(map[string]string{"type": "sse", "event": event.String()}).Observe(percent)
}

func WebsocketEventSent(event events.Name, token string) {
	eventsQuantity.With(map[string]string{"type": "websocket", "event": event.String(), "token_name": token}).Inc()
}

func WebsocketDroppedEvents(event events.Name, percent float64) {
	droppedEvents.With(map[string]string{"type": "websocket", "event": event.String()}).Observe(percent)
}
