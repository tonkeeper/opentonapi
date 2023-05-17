package sse

import (
	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
)

type Event struct {
	Name    events.Name
	EventID int64  `json:"event_id"`
	Data    []byte `json:"data"`
}
