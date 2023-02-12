package litestorage

import (
	"context"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"go.uber.org/zap"
)

type LiteStorage struct {
	client                  *liteapi.Client
	transactionsIndex       map[tongo.AccountID][]*core.Transaction
	transactionsIndexByHash map[tongo.Bits256]*core.Transaction
	blockCache              map[tongo.BlockIDExt]*tlb.Block
}

func NewLiteStorage(preloadAccounts []tongo.AccountID, log *zap.Logger) (*LiteStorage, error) {
	client, err := liteapi.NewClientWithDefaultMainnet()
	if err != nil {
		return nil, err
	}

	l := &LiteStorage{
		client:                  client,
		transactionsIndex:       make(map[tongo.AccountID][]*core.Transaction),
		transactionsIndexByHash: make(map[tongo.Bits256]*core.Transaction),
		blockCache:              make(map[tongo.BlockIDExt]*tlb.Block),
	}
	for _, a := range preloadAccounts {
		l.preloadAccount(a, log)
	}
	return l, nil
}

func (s *LiteStorage) preloadAccount(a tongo.AccountID, log *zap.Logger) error {
	ctx := context.Background()
	accountTxs, err := s.client.GetLastTransactions(ctx, a, 1000)
	if err != nil {
		return err
	}
	for _, tx := range accountTxs {
		t, err := core.ConvertTransaction(a.Workchain, tx)
		if err != nil {
			return err
		}
		s.transactionsIndexByHash[tongo.Bits256(tx.Hash())] = t
		s.transactionsIndex[a] = append(s.transactionsIndex[a], t)
	}

	return nil
}

func (s *LiteStorage) GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error) {
	blockID, _, err := s.client.LookupBlock(ctx, id, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	block, err := s.client.GetBlock(ctx, blockID)
	if err != nil {
		return nil, err
	}
	s.blockCache[blockID] = &block
	header, err := core.ConvertToBlockHeader(blockID, &block)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (s *LiteStorage) GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error) {
	tx, prs := s.transactionsIndexByHash[hash]
	if prs {
		return tx, nil
	}
	return nil, fmt.Errorf("not found tx %x", hash)
}

func (s *LiteStorage) GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error) {
	tx, err := s.GetTransaction(ctx, hash)
	if err != nil {
		return nil, err
	}
	root, err := s.findRoot(ctx, tx)
	if err != nil {
		return nil, err
	}
	trace, err := s.recursiveGetChildren(ctx, *root)
	return &trace, err
}

func (s *LiteStorage) recursiveGetChildren(ctx context.Context, tx core.Transaction) (core.Trace, error) {
	trace := core.Trace{Transaction: tx}
	for _, m := range tx.OutMsgs {
		if m.Destination == nil {
			continue
		}
		tx, err := s.searchTransactionNearBlock(ctx, *m.Destination, m.CreatedLt, tx.BlockID, false)
		if err != nil {
			return core.Trace{}, err
		}
		child, err := s.recursiveGetChildren(ctx, *tx)
		if err != nil {
			return core.Trace{}, err
		}
		trace.Children = append(trace.Children, &child)
	}
	return trace, nil
}

func (s *LiteStorage) findRoot(ctx context.Context, tx *core.Transaction) (*core.Transaction, error) {
	if tx == nil {
		return nil, fmt.Errorf("can't find root of nil transaction")
	}
	if tx.InMsg == nil || tx.InMsg.IsExtenal() || tx.InMsg.IsEmission() {
		return tx, nil
	}
	var err error
	tx, err = s.searchTransactionNearBlock(ctx, *tx.InMsg.Source, tx.InMsg.CreatedLt, tx.BlockID, true)
	if err != nil {
		return nil, err
	}

	return s.findRoot(ctx, tx)
}

func (s *LiteStorage) searchTransactionNearBlock(ctx context.Context, a tongo.AccountID, lt uint64, blockID tongo.BlockID, back bool) (*core.Transaction, error) {
	tx := s.searchTxInCache(a, lt)
	if tx != nil {
		return tx, nil
	}
	tx, err := s.searchTransactionInBlock(ctx, a, lt, blockID, back)
	if err != nil {
		if back {
			blockID.Seqno--
		} else {
			blockID.Seqno++
		}
		tx, err = s.searchTransactionInBlock(ctx, a, lt, blockID, back)
		if err != nil {
			return nil, err
		}

	}
	s.transactionsIndex[a] = append(s.transactionsIndex[a], tx)
	return tx, nil
}

func (s *LiteStorage) searchTransactionInBlock(ctx context.Context, a tongo.AccountID, lt uint64, blockID tongo.BlockID, back bool) (*core.Transaction, error) {
	blockIDExt, _, err := s.client.LookupBlock(ctx, blockID, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	block, prs := s.blockCache[blockIDExt]
	if !prs {
		b, err := s.client.GetBlock(ctx, blockIDExt)
		if err != nil {
			return nil, err
		}
		s.blockCache[blockIDExt] = &b
		block = &b
	}
	for _, tx := range block.AllTransactions() {
		if tx.AccountAddr != a.Address {
			continue
		}
		if !back && tx.Msgs.InMsg.Exists && tx.Msgs.InMsg.Value.Value.Info.IntMsgInfo.CreatedLt == lt {
			return core.ConvertTransaction(a.Workchain, tongo.Transaction{BlockID: blockIDExt, Transaction: tx})
		}
		if back {
			for _, m := range tx.Msgs.OutMsgs.Values() {
				if m.Value.Info.IntMsgInfo.CreatedLt == lt {
					return core.ConvertTransaction(a.Workchain, tongo.Transaction{BlockID: blockIDExt, Transaction: tx})
				}
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *LiteStorage) searchTxInCache(a tongo.AccountID, lt uint64) *core.Transaction {
	return nil
}
