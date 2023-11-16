package sources

import (
	"context"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"
)

// BlockchainSource notifies about transactions in the TON blockchain.
type BlockchainSource struct {
	dispatcher txDispatcher
	client     *liteapi.Client
	logger     *zap.Logger
}

type txDispatcher interface {
	RegisterSubscriber(fn DeliveryFn, options SubscribeToTransactionsOptions) CancelFn
	Run(ctx context.Context) chan TransactionEvent
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
		zap.Bool("all-operations", opts.AllOperations),
		zap.Stringers("accounts", opts.Accounts),
		zap.Strings("operations", opts.Operations))

	return b.dispatcher.RegisterSubscriber(deliverFn, opts)
}

func msgOpCodeAndName(cell *boc.Cell) (opCode *uint32, opName *abi.MsgOpName) {
	if cell.BitsAvailableForRead() < 32 {
		return nil, nil
	}
	opcode, err := cell.ReadUint(32)
	if err != nil {
		return nil, nil
	}
	msgOpCode := g.Pointer(uint32(opcode))

	cell.ResetCounters()
	name, _, err := abi.MessageDecoder(cell)
	if err != nil {
		return msgOpCode, nil
	}
	return msgOpCode, &name
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
					var msgOpCode *uint32
					var msgOpName *abi.MsgOpName
					if tx.Msgs.InMsg.Exists {
						cell := boc.Cell(tx.Msgs.InMsg.Value.Value.Body.Value)
						msgOpCode, msgOpName = msgOpCodeAndName(&cell)
					}
					ch <- TransactionEvent{
						AccountID: *tongo.NewAccountId(block.ID.Workchain, tx.AccountAddr),
						Lt:        tx.Lt,
						TxHash:    tx.Hash().Hex(),
						MsgOpName: msgOpName,
						MsgOpCode: msgOpCode,
					}
				}
			}
		}
	}()
	return blockCh
}
