package sse

type Event struct {
	EventID int64  `json:"event_id"`
	Data    []byte `json:"data"`
}
