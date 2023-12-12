package sources

import (
	"context"
	"fmt"

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
	txDispatcher    txDispatcher
	blockDispatcher blockDispatcher
	client          *liteapi.Client
	logger          *zap.Logger
}

type txDispatcher interface {
	RegisterSubscriber(fn DeliveryFn, options SubscribeToTransactionsOptions) CancelFn
	Run(ctx context.Context) chan TransactionEvent
}
type blockDispatcher interface {
	RegisterSubscriber(fn DeliveryFn, options SubscribeToBlocksOptions) CancelFn
	Run(ctx context.Context) chan BlockEvent
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
		txDispatcher:    NewTransactionDispatcher(logger),
		blockDispatcher: NewBlockDispatcher(logger),
		client:          client,
		logger:          logger,
	}, nil
}

var _ BlockSource = (*BlockchainSource)(nil)
var _ TransactionSource = (*BlockchainSource)(nil)

func (b *BlockchainSource) SubscribeToTransactions(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToTransactionsOptions) CancelFn {
	b.logger.Debug("subscribe to transactions",
		zap.Bool("all-accounts", opts.AllAccounts),
		zap.Bool("all-operations", opts.AllOperations),
		zap.Stringers("accounts", opts.Accounts),
		zap.Strings("operations", opts.Operations))

	return b.txDispatcher.RegisterSubscriber(deliveryFn, opts)
}

func (b *BlockchainSource) SubscribeToBlocks(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToBlocksOptions) CancelFn {
	b.logger.Debug("subscribe to blocks",
		zap.Intp("workchain", opts.Workchain))

	return b.blockDispatcher.RegisterSubscriber(deliveryFn, opts)
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
	newBlockCh := make(chan indexer.IDandBlock)
	go func() {
		ch := b.txDispatcher.Run(ctx)
		blockCh := b.blockDispatcher.Run(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case block := <-newBlockCh:
				blockCh <- BlockEvent{
					Workchain: block.ID.Workchain,
					Shard:     fmt.Sprintf("%x", block.ID.Shard),
					Seqno:     block.ID.Seqno,
					RootHash:  block.ID.RootHash.Hex(),
					FileHash:  block.ID.FileHash.Hex(),
				}
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
	return newBlockCh
}
