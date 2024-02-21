package sources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
)

type mockTxDispatcher struct {
	ch chan TransactionEvent
}

func (m *mockTxDispatcher) RegisterSubscriber(fn DeliveryFn, options SubscribeToTransactionsOptions) CancelFn {
	panic("implement me")
}
func (m *mockTxDispatcher) Run(ctx context.Context) chan TransactionEvent {
	return m.ch
}

type mockBlockDispatcher struct {
	ch chan BlockEvent
}

func (m *mockBlockDispatcher) RegisterSubscriber(fn DeliveryFn, options SubscribeToBlockHeadersOptions) CancelFn {
	panic("implement me")
}
func (m *mockBlockDispatcher) Run(ctx context.Context) chan BlockEvent {
	return m.ch
}

func TestBlockchainSource_Run(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)

	tests := []struct {
		name           string
		blockID        string
		wantTxEvents   []TransactionEvent
		wantBlockEvent BlockEvent
	}{
		{
			blockID: "(0,8000000000000000,40007846)",
			wantBlockEvent: BlockEvent{
				Workchain: 0,
				Shard:     "8000000000000000",
				Seqno:     40007846,
				RootHash:  "7e1ff48976668e4841b02c4b034388941915b1ce99c51a5a6a93455248aac07f",
				FileHash:  "2e8e0a874581df44e8ff47445172120dfd4ddbd96f4cff781f2573ef5c062a30",
			},
			wantTxEvents: []TransactionEvent{

				{
					AccountID: ton.MustParseAccountID("0:623d1fbe2220bb6e1076473d548ef250a1b6aaea35bce50f2fe99c31af6169a8"),
					Lt:        42562202000001,
					TxHash:    "0c9e611b886152d5f882b46d492d6746adc4a4812ac1d6862b66c813403a9ab1",
					MsgOpCode: g.Pointer(uint32(0x41b9a0a4)),
				},
				{
					AccountID: ton.MustParseAccountID("0:1150b518b2626ad51899f98887f8824b70065456455f7fe2813f012699a4061f"),
					Lt:        42562202000003,
					TxHash:    "9bb69427ec3c41caa01f9169c39202adce8a00d5e014fb38cb572063f1a275d6",
					MsgOpName: g.Pointer("JettonTransfer"),
					MsgOpCode: g.Pointer(uint32(0xf8a7ea5)),
				},
				{
					AccountID: ton.MustParseAccountID("0:779dcc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e"),
					Lt:        42562202000005,
					TxHash:    "f041fbc9d3ecbcd59be218b3e563f0499baa3c670a2caa4644be7c1ca3bfde09",
					MsgOpName: g.Pointer("JettonNotify"),
					MsgOpCode: g.Pointer(uint32(0x7362d09c)),
				},
				{
					AccountID: ton.MustParseAccountID("0:ed6473ad55669ff54214eab9a32cf34b4c6c4d2cda20bcae7151994c46067043"),
					Lt:        42562202000007,
					TxHash:    "203a9fa3cf95353f06989a5227772f82d654a0ad64911c67a1a3b763c1bccdaa",
					MsgOpCode: g.Pointer(uint32(0xfcf9e58f)),
				},
				{
					AccountID: ton.MustParseAccountID("0:62cf96996cf63631ac1d5487c807a02ed5fba102198fd12b44b2b3cb7fcfc51b"),
					Lt:        42562202000009,
					TxHash:    "8cd2c2a9e299864f2d355a0e5ac2ea702eea0e4c57f90f060bc702893d56008e",
					MsgOpCode: g.Pointer(uint32(0x3ebe5431)),
				},
				{
					AccountID: ton.MustParseAccountID("0:ed6473ad55669ff54214eab9a32cf34b4c6c4d2cda20bcae7151994c46067043"),
					Lt:        42562202000011,
					TxHash:    "5bd8434e0a109442eb0697497b66e4a32f56b1fe6ea73cc819f0d443eb4fa5be",
					MsgOpCode: g.Pointer(uint32(0x56dfeb8a)),
				},
				{
					AccountID: ton.MustParseAccountID("0:fd4fc5368284b4a7fac703cc440f6fd4e284def1af3d5dfce3cde443c360684c"),
					Lt:        42562202000013,
					TxHash:    "f9e4fa3ade5c1eb2ac6638c1ee2757b451910c1f88957f3fa8483b386291700a",
					MsgOpName: g.Pointer("JettonInternalTransfer"),
					MsgOpCode: g.Pointer(uint32(0x178d4519)),
				},
				{
					AccountID: ton.MustParseAccountID("0:623d1fbe2220bb6e1076473d548ef250a1b6aaea35bce50f2fe99c31af6169a8"),
					Lt:        42562202000015,
					TxHash:    "2095354e54e510bcf5df57e14bc608ffab6b3d9b4a64345c122246ba0534d195",
					MsgOpName: g.Pointer("Excess"),
					MsgOpCode: g.Pointer(uint32(0xd53276db)),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockID, err := tongo.ParseBlockID(tt.blockID)
			require.Nil(t, err)

			mockDisp := &mockTxDispatcher{
				ch: make(chan TransactionEvent),
			}
			blockDisp := &mockBlockDispatcher{
				ch: make(chan BlockEvent, 10),
			}
			b := &BlockchainSource{
				logger:          zap.L(),
				txDispatcher:    mockDisp,
				blockDispatcher: blockDisp,
			}
			blockCh := b.Run(context.Background())
			extID, _, err := cli.LookupBlock(context.Background(), blockID, 1, nil, nil)
			require.Nil(t, err)
			block, err := cli.GetBlock(context.Background(), extID)
			require.Nil(t, err)

			txCounts := len(block.AllTransactions())

			blockCh <- indexer.IDandBlock{
				ID:    extID,
				Block: &block,
			}

			var events []TransactionEvent
			for i := 0; i < txCounts; i++ {
				event := <-mockDisp.ch
				events = append(events, event)
			}
			require.Equal(t, tt.wantTxEvents, events)

			event := <-blockDisp.ch
			require.Equal(t, tt.wantBlockEvent, event)
		})
	}
}
