package sources

import (
	"sync"

	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

// TraceDispatcher implements the fan-out pattern reading a TraceEvent from a single channel
// and delivering it to multiple subscribers.
type TraceDispatcher struct {
	logger *zap.Logger

	mu          sync.RWMutex
	accounts    map[tongo.AccountID]map[subscriberID]DeliveryFn
	allAccounts map[subscriberID]DeliveryFn
	options     map[subscriberID]SubscribeToTraceOptions
	currentID   subscriberID
}

// TraceEventData represents a notification about a completed trace.
// This is part of our API contract with subscribers.
type TraceEventData struct {
	AccountIDs []tongo.AccountID `json:"accounts"`
	Hash       string            `json:"hash"`
}

// NewTraceDispatcher creates a new instance of TraceDispatcher.
func NewTraceDispatcher(logger *zap.Logger) *TraceDispatcher {
	return &TraceDispatcher{
		logger:      logger,
		accounts:    map[tongo.AccountID]map[subscriberID]DeliveryFn{},
		allAccounts: map[subscriberID]DeliveryFn{},
		options:     map[subscriberID]SubscribeToTraceOptions{},
		currentID:   1,
	}
}

func (disp *TraceDispatcher) dispatch(accountIDs []tongo.AccountID, event []byte) {
	disp.mu.RLock()
	defer disp.mu.RUnlock()

	// we don't want to deliver the same event twice to the same subscriber.
	delivered := make(map[subscriberID]struct{}, len(disp.allAccounts))

	for subscriberID, deliveryFn := range disp.allAccounts {
		delivered[subscriberID] = struct{}{}
		deliveryFn(event)
	}
	for _, account := range accountIDs {
		subscribers := disp.accounts[account]
		for subscriberID, deliveryFn := range subscribers {
			if _, ok := delivered[subscriberID]; ok {
				continue
			}
			delivered[subscriberID] = struct{}{}
			deliveryFn(event)
		}
	}
}

func (disp *TraceDispatcher) RegisterSubscriber(fn DeliveryFn, options SubscribeToTraceOptions) CancelFn {
	disp.mu.Lock()
	defer disp.mu.Unlock()

	id := disp.currentID
	disp.currentID += 1
	disp.options[id] = options

	if options.AllAccounts {
		disp.allAccounts[id] = fn
		return func() { disp.unsubscribe(id) }
	}

	for _, account := range options.Accounts {
		subscribers, ok := disp.accounts[account]
		if !ok {
			subscribers = map[subscriberID]DeliveryFn{id: fn}
			disp.accounts[account] = subscribers
		}
		subscribers[id] = fn
	}

	return func() { disp.unsubscribe(id) }
}

func (disp *TraceDispatcher) unsubscribe(id subscriberID) {
	disp.mu.Lock()
	defer disp.mu.Unlock()

	options, ok := disp.options[id]
	if !ok {
		return
	}
	delete(disp.options, id)
	if options.AllAccounts {
		delete(disp.allAccounts, id)
		return
	}
	for _, account := range options.Accounts {
		subscribers, ok := disp.accounts[account]
		if !ok {
			continue
		}
		delete(subscribers, id)
		if len(subscribers) == 0 {
			delete(disp.accounts, account)
		}
	}
}
