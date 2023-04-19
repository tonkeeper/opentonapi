package sources

import (
	"context"
	"time"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"
)

// BlockchainSource notifies about transactions in the TON blockchain.
type BlockchainSource struct {
	dispatcher *TransactionDispatcher
	client     *liteapi.Client
	logger     *zap.Logger
}

func NewBlockchainSource(logger *zap.Logger, servers []config.LiteServer) (*BlockchainSource, error) {
	var err error
	var client *liteapi.Client
	if len(servers) == 0 {
		client, err = liteapi.NewClientWithDefaultMainnet()
	} else {
		client, err = liteapi.NewClient(liteapi.WithLiteServers(servers))
	}
	if err != nil {
		return nil, err
	}
	return &BlockchainSource{
		dispatcher: NewTransactionDispatcher(logger),
		client:     client,
		logger:     logger,
	}, nil
}

var _ TransactionSource = (*BlockchainSource)(nil)

func (b *BlockchainSource) SubscribeToTransactions(deliverFn DeliveryTxFn, opts SubscribeToTransactionsOptions) CancelFn {
	b.logger.Debug("subscribe to transactions",
		zap.Bool("all-accounts", opts.AllAccounts),
		zap.Stringers("accounts", opts.Accounts))

	return b.dispatcher.RegisterSubscriber(deliverFn, opts)
}

func (b *BlockchainSource) Run(ctx context.Context) {
	var chunk *chunk
	idx := indexer{cli: b.client}
	for {
		time.Sleep(200 * time.Millisecond)
		info, err := b.client.GetMasterchainInfo(ctx)
		if err != nil {
			b.logger.Error("failed to get masterchain info", zap.Error(err))
			continue
		}
		chunk, err = idx.initChunk(info.Last.Seqno)
		if err != nil {
			b.logger.Error("failed to get init chunk", zap.Error(err))
			continue
		}
		break
	}

	ch := b.dispatcher.Run(ctx)

	for {
		time.Sleep(500 * time.Millisecond)
		next, err := idx.next(chunk)
		if err != nil {
			if isBlockNotReadyError(err) {
				continue
			}
			b.logger.Error("failed to get next chunk", zap.Error(err))
			continue
		}
		for _, block := range next.blocks {
			transactions := block.Block.AllTransactions()
			for _, tx := range transactions {
				ch <- TransactionEvent{
					AccountID: *tongo.NewAccountId(block.ID.Workchain, tx.AccountAddr),
					Lt:        tx.Lt,
					TxHash:    tx.Hash().Hex(),
				}
			}
		}
		chunk = next
	}
}
