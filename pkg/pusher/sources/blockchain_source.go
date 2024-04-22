package sources

import (
	"context"
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
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
	RegisterSubscriber(fn DeliveryFn, options SubscribeToBlockHeadersOptions) CancelFn
	Run(ctx context.Context) chan BlockEvent
}

func NewBlockchainSource(logger *zap.Logger, cli *liteapi.Client) *BlockchainSource {
	return &BlockchainSource{
		txDispatcher:    NewTransactionDispatcher(logger),
		blockDispatcher: NewBlockDispatcher(logger),
		client:          cli,
		logger:          logger,
	}
}

var _ BlockHeadersSource = (*BlockchainSource)(nil)
var _ TransactionSource = (*BlockchainSource)(nil)

func (b *BlockchainSource) SubscribeToTransactions(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToTransactionsOptions) CancelFn {
	b.logger.Debug("subscribe to transactions",
		zap.Bool("all-accounts", opts.AllAccounts),
		zap.Bool("all-operations", opts.AllOperations),
		zap.Stringers("accounts", opts.Accounts),
		zap.Strings("operations", opts.Operations))

	return b.txDispatcher.RegisterSubscriber(deliveryFn, opts)
}

func (b *BlockchainSource) SubscribeToBlockHeaders(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToBlockHeadersOptions) CancelFn {
	b.logger.Debug("subscribe to blocks",
		zap.Intp("workchain", opts.Workchain))

	return b.blockDispatcher.RegisterSubscriber(deliveryFn, opts)
}

func msgOpCodeAndName(msg tlb.Message, cell *boc.Cell) (opCode *uint32, opName *abi.MsgOpName) {
	if msg.Info.IntMsgInfo != nil {
		tag, name, _, _ := abi.InternalMessageDecoder(cell, nil)
		return tag, name
	}
	if msg.Info.ExtInMsgInfo != nil {
		tag, name, _, _ := abi.ExtInMessageDecoder(cell, nil)
		return tag, name
	}
	if msg.Info.ExtOutMsgInfo != nil {
		tag, name, _, _ := abi.ExtOutMessageDecoder(cell, nil, msg.Info.ExtOutMsgInfo.Dest)
		return tag, name
	}
	return nil, nil
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
						msgOpCode, msgOpName = msgOpCodeAndName(tx.Msgs.InMsg.Value.Value, &cell)
					}
					ch <- TransactionEvent{
						AccountID: *ton.NewAccountID(block.ID.Workchain, tx.AccountAddr),
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
