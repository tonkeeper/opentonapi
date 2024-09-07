package opentonadapter

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/arnac-io/opentonapi/pkg/api"
	"github.com/arnac-io/opentonapi/pkg/blockchain/indexer"
	"github.com/arnac-io/opentonapi/pkg/core"
	"github.com/arnac-io/opentonapi/pkg/litestorage"
	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
)

func mustNewCache[T any](t *testing.T) litestorage.ICache[T] {
	cache, err := litestorage.NewCache[T](100000000000)
	require.NoError(t, err)
	return cache
}

func mustNewLiteStorage(t *testing.T) *litestorage.LiteStorage {
	client, err := liteapi.NewClientWithDefaultMainnet()
	require.NoError(t, err)
	storage, err := litestorage.NewLiteStorage(
		zap.L(),
		client,
		litestorage.WithTrackAllAccounts(),
		litestorage.WithTransactionsIndexByHash(mustNewCache[core.Transaction](t)),
		litestorage.WithTransactionsByInMsgLT(mustNewCache[string](t)),
	)
	require.NoError(t, err)
	return storage
}

func TestIndexer(t *testing.T) {
	type indexerTest struct {
		name           string
		blockNumber    uint32
		expectedBlocks int
	}

	tests := []indexerTest{
		{
			name:           "Bad block on old openton version",
			blockNumber:    39972808,
			expectedBlocks: 4,
		},
		{
			name:           "single shard single block",
			blockNumber:    39919901,
			expectedBlocks: 1,
		},
		{
			name:           "no new blocks",
			blockNumber:    39919900,
			expectedBlocks: 0,
		},
		{
			name:           "two shards merged",
			blockNumber:    39912133,
			expectedBlocks: 1,
		},
		{
			name:           "two shards - one block only",
			blockNumber:    39912123,
			expectedBlocks: 1,
		},
		{
			name:           "two shards - one block each",
			blockNumber:    39912124,
			expectedBlocks: 2,
		},
		{
			name:           "two shards - after a split",
			blockNumber:    39896761,
			expectedBlocks: 2,
		},
		{
			name:           "three shards - after a split",
			blockNumber:    39900803,
			expectedBlocks: 3,
		},
		{
			name:           "more blocks than shards",
			blockNumber:    39900778,
			expectedBlocks: 4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := liteapi.NewClientWithDefaultMainnet()
			require.NoError(t, err)
			opentonIndexer := indexer.New(zap.L(), client)
			masterBlock, blocks, err := opentonIndexer.GetBlocksFromMasterBlock(test.blockNumber)
			require.NoError(t, err)
			require.Len(t, blocks, test.expectedBlocks)
			require.Equal(t, test.blockNumber, masterBlock.ID.BlockID.Seqno)
		})
	}
}

func TestLiteStorage(t *testing.T) {
	ctx := context.Background()

	type liteStorageTest struct {
		name                    string
		blockNumber             uint32
		expectedTransactionHash string
	}

	tests := []liteStorageTest{
		{
			name:                    "Sanity check",
			blockNumber:             39919901,
			expectedTransactionHash: "CSSVGDSLiok8NYUzj35p5NmawlH7qWhr+DsclKYTqcY=",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := liteapi.NewClientWithDefaultMainnet()
			require.NoError(t, err)
			opentonIndexer := indexer.New(zap.L(), client)
			_, blocks, err := opentonIndexer.GetBlocksFromMasterBlock(test.blockNumber)
			require.NoError(t, err)

			storage := mustNewLiteStorage(t)
			for _, block := range blocks {
				storage.ParseBlock(ctx, block)
			}
			time.Sleep(1 * time.Second)
			transaction, err := storage.GetTransaction(ctx, tongo.MustParseHash(test.expectedTransactionHash))
			require.NoError(t, err)
			require.NotNil(t, transaction)
		})
	}
}

func TestHandler(t *testing.T) {
	ctx := context.Background()

	type handlerTest struct {
		name              string
		blockNumbers      []uint32
		transactionHashes []string
		isTraceInProgress bool
	}

	tests := []handlerTest{
		{
			name:              "Simple Ton transfer, 2 transaction in same block",
			blockNumbers:      []uint32{39955819},
			transactionHashes: []string{"7QN3fYcRckwh0IOcJFT/FSb2UbUDHdl3GZ1IYmJIa6s=", "BNGho3lsRUwIfre5S4Cwtp+DOD5wMBUmQ8q4r4oevok="},
			isTraceInProgress: false,
		},
		{
			name:              "Simple Ton transfer, 2 transactions in different blocks",
			blockNumbers:      []uint32{39957151, 39957156},
			transactionHashes: []string{"irv3D7NwfqUnMZBHTQUkmspVJU5qSQnikwpNoq7BGxE=", "uUS+L16jLV6ooTjDoSOYJXkUW9QhWS332TPOnb7Mc60="},
			isTraceInProgress: false,
		},
		{
			name:              "Simple Ton transfer, in progress",
			blockNumbers:      []uint32{39957151},
			transactionHashes: []string{"irv3D7NwfqUnMZBHTQUkmspVJU5qSQnikwpNoq7BGxE=", "uUS+L16jLV6ooTjDoSOYJXkUW9QhWS332TPOnb7Mc60="},
			isTraceInProgress: true,
		},
		{
			name:              "Jetton transfer",
			blockNumbers:      []uint32{39991652, 39991666, 39991669, 39991672, 39991679},
			transactionHashes: []string{"p00rAG3PvR8pXJAifzI4g8nCoi5JiFuad4XIJLrtYYc=", "N5C3R6OKKjNFriAn382CEiuc0gzKvjDy1W6bYJCUZJI=", "sqmpBcrAiiYtntlA3Vb3wbOU8SCmY7ke//uE/S6bHiQ=", "ZyESzfANa4hD2vT+hQefGN764eXkp/NLpKSWeUW5Nw0=", "up7LCt2BwthFXBNZW6gUHX5qjXn4qqMM8hwAGqhm8WU="},
			isTraceInProgress: false,
		},
		{
			name:              "Jetton transfer in progress",
			blockNumbers:      []uint32{39991652, 39991666},
			transactionHashes: []string{"p00rAG3PvR8pXJAifzI4g8nCoi5JiFuad4XIJLrtYYc=", "N5C3R6OKKjNFriAn382CEiuc0gzKvjDy1W6bYJCUZJI=", "sqmpBcrAiiYtntlA3Vb3wbOU8SCmY7ke//uE/S6bHiQ=", "ZyESzfANa4hD2vT+hQefGN764eXkp/NLpKSWeUW5Nw0=", "up7LCt2BwthFXBNZW6gUHX5qjXn4qqMM8hwAGqhm8WU="},
			isTraceInProgress: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := liteapi.NewClientWithDefaultMainnet()
			require.NoError(t, err)
			opentonIndexer := indexer.New(zap.L(), client)
			liteStorage := mustNewLiteStorage(t)

			for _, blockNumber := range test.blockNumbers {
				_, blocks, err := opentonIndexer.GetBlocksFromMasterBlock(blockNumber)
				require.NoError(t, err)
				require.Greater(t, len(blocks), 0)
				for _, block := range blocks {
					liteStorage.ParseBlock(ctx, block)
				}
			}

			handler, err := api.NewHandler(zap.L(), api.WithStorage(liteStorage), api.WithExecutor(liteStorage))
			require.NoError(t, err)

			for _, transactionHash := range test.transactionHashes {
				_, _, found, err := handler.GetEventAndTraceByHash(ctx, transactionHash)
				require.NoError(t, err)
				if test.isTraceInProgress {
					require.False(t, found)
				} else {
					require.True(t, found)
				}
			}
		})
	}
}
