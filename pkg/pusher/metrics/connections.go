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
		"token_name",
	},
)

func OpenWebsocketConnection(token string) {
	openConnections.With(map[string]string{"type": "websocket", "token_name": token}).Inc()
}

func CloseWebsocketConnection(token string) {
	openConnections.With(map[string]string{"type": "websocket", "token_name": token}).Dec()
}

func OpenSseConnection(token string) {
	openConnections.With(map[string]string{"type": "sse", "token_name": token}).Inc()
}

func CloseSseConnection(token string) {
	openConnections.With(map[string]string{"type": "sse", "token_name": token}).Dec()
}
