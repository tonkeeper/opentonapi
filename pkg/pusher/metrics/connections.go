package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var openConnections = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "streaming_api_open_connections",
	},
	[]string{
		"type",
	},
)

func OpenWebsocketConnection() {
	openConnections.With(map[string]string{"type": "websocket"}).Inc()
}

func CloseWebsocketConnection() {
	openConnections.With(map[string]string{"type": "websocket"}).Dec()
}

func OpenSseConnection() {
	openConnections.With(map[string]string{"type": "sse"}).Inc()
}

func CloseSseConnection() {
	openConnections.With(map[string]string{"type": "sse"}).Dec()
}
