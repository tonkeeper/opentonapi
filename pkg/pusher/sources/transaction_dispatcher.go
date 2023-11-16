package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"go.uber.org/zap"
)

type subscriberID int64

// TransactionEvent is a notification event about a new transaction between a TransactionSource instance and a dispatcher.
type TransactionEvent struct {
	AccountID tongo.AccountID
	Lt        uint64
	TxHash    string
	// MsgOpName is an operation name taken from the first 4 bytes of tx.InMsg.Body.
	MsgOpName *abi.MsgOpName
	// MsgOpCode is an operation code taken from the first 4 bytes of tx.InMsg.Body.
	MsgOpCode *uint32
}

type deliveryFn func(eventData []byte, msgOpName *abi.MsgOpName, msgOpCode *uint32)

// TransactionDispatcher implements the fan-out pattern reading a TransactionEvent from a single channel
// and delivering it to multiple subscribers.
type TransactionDispatcher struct {
	logger *zap.Logger

	mu          sync.RWMutex
	accounts    map[tongo.AccountID]map[subscriberID]deliveryFn
	allAccounts map[subscriberID]deliveryFn
	options     map[subscriberID]SubscribeToTransactionsOptions
	currentID   subscriberID
}

func NewTransactionDispatcher(logger *zap.Logger) *TransactionDispatcher {
	return &TransactionDispatcher{
		logger:      logger,
		accounts:    map[tongo.AccountID]map[subscriberID]deliveryFn{},
		allAccounts: map[subscriberID]deliveryFn{},
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
				disp.dispatch(&tx, event.MsgOpName, event.MsgOpCode)
			}
		}
	}()
	return ch
}

func (disp *TransactionDispatcher) dispatch(tx *TransactionEventData, msgOpName *abi.MsgOpName, msgOpCode *uint32) {
	eventData, err := json.Marshal(tx)
	if err != nil {
		disp.logger.Error("json.Marshal() failed: %v", zap.Error(err))
		return
	}
	disp.mu.RLock()
	defer disp.mu.RUnlock()

	for _, deliveryFn := range disp.allAccounts {
		deliveryFn(eventData, msgOpName, msgOpCode)
	}
	subscribers := disp.accounts[tx.AccountID]
	for _, deliveryFn := range subscribers {
		deliveryFn(eventData, msgOpName, msgOpCode)
	}
}

func createDeliveryFnBasedOnOptions(fn DeliveryFn, options SubscribeToTransactionsOptions) deliveryFn {
	if options.AllOperations {
		return func(eventData []byte, msgOpName *abi.MsgOpName, msgOpCode *uint32) {
			fn(eventData)
		}
	}
	ops := make(map[abi.MsgOpName]struct{}, len(options.Operations))
	for _, op := range options.Operations {
		ops[op] = struct{}{}
	}
	return func(eventData []byte, msgOpName *abi.MsgOpName, msgOpCode *uint32) {
		if msgOpName != nil {
			if _, ok := ops[*msgOpName]; ok {
				fn(eventData)
				return
			}
		}
		if msgOpCode != nil {
			if _, ok := ops[fmt.Sprintf("0x%08x", *msgOpCode)]; ok {
				fn(eventData)
				return
			}
		}
	}
}

func (disp *TransactionDispatcher) RegisterSubscriber(fn DeliveryFn, options SubscribeToTransactionsOptions) CancelFn {
	disp.mu.Lock()
	defer disp.mu.Unlock()

	id := disp.currentID
	disp.currentID += 1
	disp.options[id] = options

	if options.AllAccounts {
		disp.allAccounts[id] = createDeliveryFnBasedOnOptions(fn, options)
		return func() { disp.unsubscribe(id) }
	}

	for _, account := range options.Accounts {
		subscribers, ok := disp.accounts[account]
		if !ok {
			subscribers = make(map[subscriberID]deliveryFn, 1)
			disp.accounts[account] = subscribers
		}
		subscribers[id] = createDeliveryFnBasedOnOptions(fn, options)
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
