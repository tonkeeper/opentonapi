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
		"token_name",
	},
)

func SseEventSent(event events.Name, token string) {
	eventsQuantity.With(map[string]string{"type": "sse", "event": event.String(), "token_name": token}).Inc()
}

func WebsocketEventSent(event events.Name, token string) {
	eventsQuantity.With(map[string]string{"type": "websocket", "event": event.String(), "token_name": token}).Inc()
}
