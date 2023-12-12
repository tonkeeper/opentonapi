package sources

import (
	"context"
	"encoding/json"
	"sync"

	"go.uber.org/zap"
)

// BlockDispatcher tracks all subscribers and works as a fan-out queue:
// on receiving a new block, BlockDispatcher sends a notification about it to all subscribers.
type BlockDispatcher struct {
	logger *zap.Logger

	// mu protects "subscribes" and "currentID" fields.
	mu         sync.RWMutex
	currentID  subscriberID
	subscribes map[subscriberID]blockDeliveryFn
}

type blockDeliveryFn func(eventData []byte, workchain int)

func NewBlockDispatcher(logger *zap.Logger) *BlockDispatcher {
	return &BlockDispatcher{
		logger:     logger,
		currentID:  1,
		subscribes: map[subscriberID]blockDeliveryFn{},
	}
}

func (disp *BlockDispatcher) Run(ctx context.Context) chan BlockEvent {
	ch := make(chan BlockEvent, 100)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ch:
				disp.logger.Debug("handling block",
					zap.String("block", event.String()))
				disp.dispatch(&event)
			}
		}
	}()
	return ch
}

func (disp *BlockDispatcher) dispatch(event *BlockEvent) {
	eventData, err := json.Marshal(event)
	if err != nil {
		disp.logger.Error("json.Marshal() failed: %v", zap.Error(err))
		return
	}
	disp.mu.RLock()
	defer disp.mu.RUnlock()

	for _, deliveryFn := range disp.subscribes {
		deliveryFn(eventData, int(event.Workchain))
	}
}

func (disp *BlockDispatcher) RegisterSubscriber(fn DeliveryFn, opts SubscribeToBlocksOptions) CancelFn {
	disp.mu.Lock()
	defer disp.mu.Unlock()

	id := disp.currentID
	disp.currentID += 1

	disp.subscribes[id] = createBlockDeliveryFnBasedOnOptions(fn, opts)
	return func() {
		disp.unsubscribe(id)
	}
}

func createBlockDeliveryFnBasedOnOptions(fn DeliveryFn, options SubscribeToBlocksOptions) blockDeliveryFn {
	if options.Workchain == nil {
		return func(eventData []byte, workchain int) {
			fn(eventData)
		}
	}
	return func(eventData []byte, workchain int) {
		if workchain == *options.Workchain {
			fn(eventData)
		}
	}
}

func (disp *BlockDispatcher) unsubscribe(id subscriberID) {
	disp.mu.Lock()
	defer disp.mu.Unlock()
	delete(disp.subscribes, id)
}
