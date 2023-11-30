package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"github.com/tonkeeper/opentonapi/pkg/blockchain"
)

var testAccount1 = ton.MustParseAccountID("0:0a95e1d4ebe7860d051f8b861730dbdee1440fd11180211914e0089146580351")
var testAccount2 = ton.MustParseAccountID("0:0a95e1d4ebe7860d051f8b861730dbdee1440fd11180211914e0089146580352")

func TestMemPool_Run(t *testing.T) {

	mempool := NewMemPool(zap.L())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := mempool.Run(ctx)
	const eventsNumber = 10

	var wg sync.WaitGroup
	wg.Add(eventsNumber * 2)

	// subscribe to mempool events
	eventDataCh := make(chan []byte, eventsNumber*2)
	cancelFn, err := mempool.SubscribeToMessages(context.Background(), func(eventData []byte) {
		eventDataCh <- eventData
		wg.Done()
	}, SubscribeToMempoolOptions{})
	require.Nil(t, err)

	// subscribe to mempool events with emulation
	emulationEventDataCh := make(chan []byte, eventsNumber)
	emulationCancelFn, err := mempool.SubscribeToMessages(context.Background(), func(eventData []byte) {
		emulationEventDataCh <- eventData
		wg.Done()
	}, SubscribeToMempoolOptions{Accounts: []tongo.AccountID{testAccount1}})
	require.Nil(t, err)

	defer cancelFn()
	defer emulationCancelFn()

	var expected [][]byte
	var emulationExpected [][]byte

	for i := 0; i < eventsNumber; i++ {
		payload := []byte(fmt.Sprintf("payload-%d", i))
		ch <- blockchain.ExtInMsgCopy{
			MsgBoc:  base64.StdEncoding.EncodeToString(payload),
			Payload: payload,
		}
		eventData, err := json.Marshal(MessageEventData{BOC: payload})
		require.Nil(t, err)
		expected = append(expected, eventData)

		emPayload := []byte(fmt.Sprintf("emulation-payload-%d", i))
		ch <- blockchain.ExtInMsgCopy{
			MsgBoc:   base64.StdEncoding.EncodeToString(emPayload),
			Payload:  emPayload,
			Accounts: map[tongo.AccountID]struct{}{testAccount1: {}},
		}
		eventData, err = json.Marshal(EmulationMessageEventData{BOC: emPayload, InvolvedAccounts: []tongo.AccountID{testAccount1}})
		require.Nil(t, err)
		emulationExpected = append(emulationExpected, eventData)
	}
	wg.Wait()
	close(eventDataCh)
	close(emulationEventDataCh)

	var events [][]byte
	for data := range eventDataCh {
		events = append(events, data)
	}
	require.Equal(t, expected, events)

	events = [][]byte{}
	for data := range emulationEventDataCh {
		events = append(events, data)
	}
	require.Equal(t, emulationExpected, events)

}

func compareSubscribers(t *testing.T, expected []subscriberID, subscribers map[subscriberID]mempoolDeliveryFn) {
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
		cancel, err := mempool.SubscribeToMessages(context.Background(), func(eventData []byte) {}, SubscribeToMempoolOptions{})
		require.Nil(t, err)
		cancelFns[subID] = cancel
	}
	for i := 0; i < 5; i++ {
		subID := mempool.currentID
		options := SubscribeToMempoolOptions{Accounts: []tongo.AccountID{{}}}
		cancel, err := mempool.SubscribeToMessages(context.Background(), func(eventData []byte) {}, options)
		require.Nil(t, err)
		cancelFns[subID] = cancel
	}
	compareSubscribers(t, []subscriberID{1, 2, 3, 4, 5}, mempool.regularSubscribers)
	compareSubscribers(t, []subscriberID{6, 7, 8, 9, 10}, mempool.emulationSubscribers)

	cancelFns[3]()
	compareSubscribers(t, []subscriberID{1, 2, 4, 5}, mempool.regularSubscribers)
	compareSubscribers(t, []subscriberID{6, 7, 8, 9, 10}, mempool.emulationSubscribers)

	cancelFns[2]()
	cancelFns[4]()
	cancelFns[9]()
	cancelFns[10]()
	compareSubscribers(t, []subscriberID{1, 5}, mempool.regularSubscribers)
	compareSubscribers(t, []subscriberID{6, 7, 8}, mempool.emulationSubscribers)

	cancelFns[1]()
	cancelFns[1]()
	cancelFns[6]()
	cancelFns[6]()
	compareSubscribers(t, []subscriberID{5}, mempool.regularSubscribers)
	compareSubscribers(t, []subscriberID{7, 8}, mempool.emulationSubscribers)
}

func Test_createMempoolDeliveryFnBasedOnOptions(t *testing.T) {
	tests := []struct {
		name       string
		opts       SubscribeToMempoolOptions
		callFn     func(fn mempoolDeliveryFn)
		wantCalled bool
	}{
		{
			name: "no accounts",
			opts: SubscribeToMempoolOptions{Accounts: nil},
			callFn: func(fn mempoolDeliveryFn) {
				fn([]byte("event"), nil)
			},
			wantCalled: true,
		},
		{
			name: "involved account in options",
			opts: SubscribeToMempoolOptions{Accounts: []tongo.AccountID{testAccount1}},
			callFn: func(fn mempoolDeliveryFn) {
				fn([]byte("event"), map[tongo.AccountID]struct{}{testAccount1: {}})
			},
			wantCalled: true,
		},
		{
			name: "involved account not in options",
			opts: SubscribeToMempoolOptions{Accounts: []tongo.AccountID{testAccount2}},
			callFn: func(fn mempoolDeliveryFn) {
				fn([]byte("event"), map[tongo.AccountID]struct{}{testAccount1: {}})
			},
			wantCalled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			delivery := func(eventData []byte) {
				called = true
			}
			fn := createMempoolDeliveryFnBasedOnOptions(delivery, tt.opts)
			tt.callFn(fn)
			require.Equal(t, tt.wantCalled, called)
		})
	}
}
