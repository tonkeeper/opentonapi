package sources

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

type subscriberID int64

// TransactionEvent is a notification event about a new transaction between a TransactionSource instance and a dispatcher.
type TransactionEvent struct {
	AccountID tongo.AccountID
	Lt        uint64
	TxHash    string
}

// TransactionDispatcher implements the fan-out pattern reading a TransactionEvent from a single channel
// and delivering it to multiple subscribers.
type TransactionDispatcher struct {
	logger *zap.Logger

	mu          sync.RWMutex
	accounts    map[tongo.AccountID]map[subscriberID]DeliveryFn
	allAccounts map[subscriberID]DeliveryFn
	options     map[subscriberID]SubscribeToTransactionsOptions
	currentID   subscriberID
}

func NewTransactionDispatcher(logger *zap.Logger) *TransactionDispatcher {
	return &TransactionDispatcher{
		logger:      logger,
		accounts:    map[tongo.AccountID]map[subscriberID]DeliveryFn{},
		allAccounts: map[subscriberID]DeliveryFn{},
		options:     map[subscriberID]SubscribeToTransactionsOptions{},
		currentID:   1,
	}
}

// Run runs a dispatching loop in a dedicated goroutine and returns a channel to be used to communicate with this dispatcher.
func (disp *TransactionDispatcher) Run(ctx context.Context) chan TransactionEvent {
	ch := make(chan TransactionEvent)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ch:
				disp.logger.Debug("handling transaction",
					zap.String("account", event.AccountID.ToRaw()),
					zap.Uint64("lt", event.Lt))
				tx := TransactionEventData{
					AccountID: event.AccountID,
					Lt:        event.Lt,
					TxHash:    event.TxHash,
				}
				disp.dispatch(&tx)
			}
		}
	}()
	return ch
}

func (disp *TransactionDispatcher) dispatch(tx *TransactionEventData) {
	eventData, err := json.Marshal(tx)
	if err != nil {
		disp.logger.Error("json.Marshal() failed: %v", zap.Error(err))
		return
	}
	disp.mu.RLock()
	defer disp.mu.RUnlock()

	for _, deliveryFn := range disp.allAccounts {
		deliveryFn(eventData)
	}
	subscribers := disp.accounts[tx.AccountID]
	for _, deliveryFn := range subscribers {
		deliveryFn(eventData)
	}
}

func (disp *TransactionDispatcher) RegisterSubscriber(fn DeliveryFn, options SubscribeToTransactionsOptions) CancelFn {
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

func (disp *TransactionDispatcher) unsubscribe(id subscriberID) {
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
