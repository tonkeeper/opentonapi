package sources

import (
	"context"
	"encoding/json"
	"sync"

	"go.uber.org/zap"
)

// MemPool implements "MemPoolSource" interface
// and provides a method to subscribe to pending inbound messages.
type MemPool struct {
	logger *zap.Logger

	mu          sync.Mutex
	currentID   subscriberID
	subscribers map[subscriberID]DeliveryFn
}

func NewMemPool(logger *zap.Logger) *MemPool {
	return &MemPool{
		logger:      logger,
		currentID:   1,
		subscribers: map[subscriberID]DeliveryFn{},
	}
}

var _ MemPoolSource = (*MemPool)(nil)

func (m *MemPool) SubscribeToMessages(deliveryFn DeliveryFn) CancelFn {
	m.mu.Lock()
	defer m.mu.Unlock()

	subID := m.currentID
	m.currentID += 1
	m.subscribers[subID] = deliveryFn
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.subscribers, subID)
	}
}

// Run runs a goroutine with a fan-out event-loop that resends an incoming payload to all subscribers.
func (m *MemPool) Run(ctx context.Context) chan []byte {
	// TODO: replace with elastic channel
	ch := make(chan []byte, 100)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case payload := <-ch:
				m.sendPayloadToSubscribers(payload)
			}
		}
	}()
	return ch
}

func (m *MemPool) sendPayloadToSubscribers(payload []byte) {
	msg := MessageEventData{
		BOC: payload,
	}
	eventData, err := json.Marshal(msg)
	if err != nil {
		m.logger.Error("mempool failed to marshal payload to json",
			zap.Error(err),
			zap.ByteString("payload", payload))
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, fn := range m.subscribers {
		fn(eventData)
	}
}
