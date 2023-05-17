package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
)

var eventsQuantity = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "streaming_api_events",
	},
	[]string{
		"type",
		"event",
	},
)

func SseEventSent(event events.Name) {
	eventsQuantity.With(map[string]string{"type": "sse", "event": event.String()}).Inc()
}

func WebsocketEventSent(event events.Name) {
	eventsQuantity.With(map[string]string{"type": "websocket", "event": event.String()}).Inc()
}
