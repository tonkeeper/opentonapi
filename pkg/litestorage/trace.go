package litestorage

import (
	"context"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

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
