package litestorage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"go.uber.org/zap"

	"github.com/arnac-io/opentonapi/pkg/blockchain/indexer"
	"github.com/arnac-io/opentonapi/pkg/core"
)

func TestLiteStorage_run(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
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
		transactionsIndexByHashCache, err := NewCache[core.Transaction](10000)
		require.NoError(t, err)
		// transactionsByInMsgLTCache, err := NewCache[string](10000)
		require.NoError(t, err)
		t.Run(tt.name, func(t *testing.T) {
			s := &LiteStorage{
				logger:                  zap.L(),
				transactionsIndexByHash: transactionsIndexByHashCache,
				// transactionsByInMsgLT:   transactionsByInMsgLTCache,
				trackingAccounts: tt.trackingAccounts,
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

			for txHash := range tt.wantTxHashes {
				_, err := s.transactionsIndexByHash.Get(context.Background(), txHash)
				require.NoError(t, err)
			}
		})
	}
}

func TestLiteStorage_runBlockchainConfigUpdate(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)
	s := &LiteStorage{
		logger: zap.L(),
		client: cli,
	}
	s.runBlockchainConfigUpdate(100 * time.Millisecond)

	time.Sleep(2 * time.Second)

	configBase64 := s.blockchainConfig()
	require.NotEmpty(t, configBase64)
}

func TestLiteStorage_TrimmedConfigBase64(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)
	s := &LiteStorage{
		logger: zap.L(),
		client: cli,
	}
	conf := s.blockchainConfig()
	require.Empty(t, conf)
	conf, err = s.TrimmedConfigBase64()
	require.Nil(t, err)
	require.NotEmpty(t, conf)

	cell, err := boc.DeserializeSinglRootBase64(conf)
	require.Nil(t, err)

	config := tlb.ConfigParams{}
	err = tlb.Unmarshal(cell, &config.Config)
	require.Nil(t, err)

	allowedKeys := map[uint32]struct{}{}
	for _, key := range allowedConfigKeys {
		allowedKeys[key] = struct{}{}
	}

	for _, item := range config.Config.Items() {
		_, ok := allowedKeys[uint32(item.Key)]
		require.True(t, ok)
	}
}
