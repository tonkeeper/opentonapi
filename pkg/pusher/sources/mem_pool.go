package sources

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/core"
)

// MemPool implements "MemPoolSource" interface
// and provides a method to subscribe to pending inbound messages.
//
// MemPool supports two types of subscribers: regular and emulation.
// Regular subscriber receives a message payload once it lands in mempool.
// Emulation subscriber receives a message payload and additional emulation results with a short delay required to emulate a trace.
type MemPool struct {
	logger *zap.Logger

	mu        sync.Mutex
	currentID subscriberID
	// regularSubscribers subscribed to mempool events.
	regularSubscribers map[subscriberID]mempoolDeliveryFn
	// emulationSubscribers subscribed to mempool events with emulation.
	emulationSubscribers map[subscriberID]mempoolDeliveryFn
	accountsSubcribers   map[subscriberID]DeliveryFn
}

func NewMemPool(logger *zap.Logger) *MemPool {
	return &MemPool{
		logger:               logger,
		currentID:            1,
		regularSubscribers:   map[subscriberID]mempoolDeliveryFn{},
		emulationSubscribers: map[subscriberID]mempoolDeliveryFn{},
		accountsSubcribers:   map[subscriberID]DeliveryFn{},
	}
}

var _ MemPoolSource = (*MemPool)(nil)

type mempoolDeliveryFn func(eventData []byte, involvedAccounts map[tongo.AccountID]struct{})

func createMempoolDeliveryFnBasedOnOptions(deliveryFn DeliveryFn, opts SubscribeToMempoolOptions) mempoolDeliveryFn {
	if len(opts.Accounts) > 0 {
		return func(eventData []byte, involvedAccounts map[tongo.AccountID]struct{}) {
			if len(involvedAccounts) == 0 {
				return
			}
			for _, account := range opts.Accounts {
				if _, ok := involvedAccounts[account]; ok {
					deliveryFn(eventData)
					return
				}
			}
		}
	}
	return func(eventData []byte, involvedAccounts map[tongo.AccountID]struct{}) {
		deliveryFn(eventData)
	}
}

func (m *MemPool) SubscribeToMessages(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToMempoolOptions) (CancelFn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subID := m.currentID
	m.currentID += 1

	if len(opts.Accounts) > 0 {
		m.emulationSubscribers[subID] = createMempoolDeliveryFnBasedOnOptions(deliveryFn, opts)
	} else {
		m.regularSubscribers[subID] = createMempoolDeliveryFnBasedOnOptions(deliveryFn, opts)
	}
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.regularSubscribers, subID)
		delete(m.emulationSubscribers, subID)
	}, nil

}

func (m *MemPool) SubscribeToAccounts(ctx context.Context, deliveryFn DeliveryFn) (CancelFn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subID := m.currentID
	m.currentID += 1
	m.accountsSubcribers[subID] = deliveryFn
	cancel := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.accountsSubcribers, subID)
	}
	return cancel, nil
}

// Run runs a goroutine with a fan-out event-loop that resends an incoming payload to all subscribers.
func (m *MemPool) Run(ctx context.Context) chan blockchain.ExtInMsgCopy {
	// TODO: replace with elastic channel
	ch := make(chan blockchain.ExtInMsgCopy, 100)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msgCopy := <-ch:
				if msgCopy.IsEmulation() {
					m.sendPayloadToEmulationSubscribers(msgCopy)
					continue
				}
				m.sendPayloadToSubscribers(msgCopy.Payload)
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
	for _, fn := range m.regularSubscribers {
		fn(eventData, nil)
	}
}

func (m *MemPool) sendPayloadToEmulationSubscribers(msgCopy core.ExtInMsgCopy) {
	accountM := make(map[tongo.AccountID]struct{})
	var accounts []tongo.AccountID
	if msgCopy.Accounts != nil {
		core.Visit(msgCopy.Accounts.Trace, func(node *core.Trace) {
			accountM[node.Account] = struct{}{}
			accounts = append(accounts, node.Account)
		})
	}
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].ToRaw() < accounts[j].ToRaw()
	})
	eventData, err := json.Marshal(EmulationMessageEventData{
		BOC:              msgCopy.Payload,
		InvolvedAccounts: accounts,
	})
	if err != nil {
		m.logger.Error("mempool failed to marshal payload to json", zap.Error(err))
	}
	eventData2, err2 := json.Marshal(msgCopy.Accounts)
	if err2 != nil {
		m.logger.Warn("mempool failed to marshal account event to json", zap.Error(err2))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		for _, fn := range m.emulationSubscribers {
			fn(eventData, accountM)
		}
	}
	if err2 == nil {
		for _, fn := range m.accountsSubcribers {
			fn(eventData2)
		}
	}
}
