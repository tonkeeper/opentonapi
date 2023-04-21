package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
)

func TestMemPool_Run(t *testing.T) {
	mempool := NewMemPool(zap.L())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := mempool.Run(ctx)
	const eventsNumber = 10

	var wg sync.WaitGroup
	wg.Add(eventsNumber)

	eventDataCh := make(chan []byte, eventsNumber)
	mempool.SubscribeToMessages(func(eventData []byte) {
		eventDataCh <- eventData
		wg.Done()
	})

	var expected [][]byte
	for i := 0; i < eventsNumber; i++ {
		payload := []byte(fmt.Sprintf("payload-%d", i))
		ch <- payload
		eventData, err := json.Marshal(MessageEventData{BOC: payload})
		require.Nil(t, err)
		expected = append(expected, eventData)
	}
	wg.Wait()
	close(eventDataCh)

	var events [][]byte
	for data := range eventDataCh {
		events = append(events, data)
	}
	require.Equal(t, expected, events)

}

func compareSubscribers(t *testing.T, expected []subscriberID, subscribers map[subscriberID]DeliveryFn) {
	keys := maps.Keys(subscribers)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	require.Equal(t, expected, keys)
}

func TestMemPool_SubscribeToMessages(t *testing.T) {
	mempool := NewMemPool(zap.L())
	cancelFns := map[subscriberID]CancelFn{}
	for i := 0; i < 5; i++ {
		subID := mempool.currentID
		cancel := mempool.SubscribeToMessages(func(eventData []byte) {})
		cancelFns[subID] = cancel
	}
	compareSubscribers(t, []subscriberID{1, 2, 3, 4, 5}, mempool.subscribers)

	cancelFns[3]()
	compareSubscribers(t, []subscriberID{1, 2, 4, 5}, mempool.subscribers)

	cancelFns[2]()
	cancelFns[4]()
	compareSubscribers(t, []subscriberID{1, 5}, mempool.subscribers)

	cancelFns[1]()
	cancelFns[1]()
	compareSubscribers(t, []subscriberID{5}, mempool.subscribers)
}
