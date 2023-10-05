package sources

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
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
		logger.Warn("USING PUBLIC CONFIG for NewBlockchainSource! BE CAREFUL!")
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

func (b *BlockchainSource) SubscribeToTransactions(ctx context.Context, deliverFn DeliveryFn, opts SubscribeToTransactionsOptions) CancelFn {
	b.logger.Debug("subscribe to transactions",
		zap.Bool("all-accounts", opts.AllAccounts),
		zap.Stringers("accounts", opts.Accounts))

	return b.dispatcher.RegisterSubscriber(deliverFn, opts)
}

func (b *BlockchainSource) Run(ctx context.Context) chan indexer.IDandBlock {
	blockCh := make(chan indexer.IDandBlock)
	go func() {
		ch := b.dispatcher.Run(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case block := <-blockCh:
				transactions := block.Block.AllTransactions()
				for _, tx := range transactions {
					ch <- TransactionEvent{
						AccountID: *tongo.NewAccountId(block.ID.Workchain, tx.AccountAddr),
						Lt:        tx.Lt,
						TxHash:    tx.Hash().Hex(),
					}
				}
			}
		}
	}()
	return blockCh
}
