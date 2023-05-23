package litestorage

import (
	"context"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v2"
	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
	"github.com/tonkeeper/opentonapi/pkg/core"
)

func TestLiteStorage_run(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.Mainnet(), liteapi.FromEnvs())
	require.Nil(t, err)

	tests := []struct {
		name             string
		blockID          tongo.BlockIDExt
		trackingAccounts map[tongo.AccountID]struct{}
		wantTxHashes     map[string]struct{}
	}{
		{
			blockID: tongo.BlockIDExt{
				BlockID: tongo.BlockID{
					Workchain: 0,
					Shard:     uint64(tongo.MustParseShardID(-0x8000000000000000).Encode()),
					Seqno:     35433343,
				},
				RootHash: tongo.MustParseHash("uUXAYuzYEVp1M9PLmMh+PoaQRiw0IF4Pfc8YKoYO4GU="),
				FileHash: tongo.MustParseHash("sthyyo2Y3sQubE+hhx7MOQ0YQ+d2z4x/gkvaAq7jN1A="),
			},
			trackingAccounts: map[tongo.AccountID]struct{}{
				tongo.MustParseAccountID("0:6ccd325a858c379693fae2bcaab1c2906831a4e10a6c3bb44ee8b615bca1d220"): {},
			},
			wantTxHashes: map[string]struct{}{
				"9d224d8b784736019cbf334c38374c7448962b484a1759af8898730c80ecdaac": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &LiteStorage{
				logger:                  zap.L(),
				transactionsIndexByHash: xsync.NewTypedMapOf[tongo.Bits256, *core.Transaction](hashBits256),
				trackingAccounts:        tt.trackingAccounts,
			}
			ch := make(chan indexer.IDandBlock)
			go s.run(ch)

			block, err := cli.GetBlock(context.Background(), tt.blockID)
			require.Nil(t, err)

			ch <- indexer.IDandBlock{
				ID:    tt.blockID,
				Block: &block,
			}
			close(ch)
			time.Sleep(time.Second)

			txs := map[string]struct{}{}
			s.transactionsIndexByHash.Range(func(key tongo.Bits256, value *core.Transaction) bool {
				txs[key.Hex()] = struct{}{}
				return true
			})
			require.Equal(t, tt.wantTxHashes, txs)
		})
	}
}
