package litestorage

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

const (
	maxDepthLimit = 1024
)

func (s *LiteStorage) GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_trace").Observe(v)
	}))
	defer timer.ObserveDuration()
	tx, err := s.GetTransaction(ctx, hash)
	if err != nil {
		return nil, err
	}
	root, err := s.findRoot(ctx, tx, 0)
	if err != nil {
		return nil, err
	}
	trace, err := s.recursiveGetChildren(ctx, *root, 0)
	return &trace, err
}

func (s *LiteStorage) SearchTraces(ctx context.Context, a tongo.AccountID, limit int, beforeLT, startTime, endTime *int64, initiator bool) ([]tongo.Bits256, error) {
	return nil, nil
}

func (s *LiteStorage) recursiveGetChildren(ctx context.Context, tx core.Transaction, depth int) (core.Trace, error) {
	trace := core.Trace{Transaction: tx}
	externalMessages := make([]core.Message, 0, len(tx.OutMsgs))
	for _, m := range tx.OutMsgs {
		if m.Destination == nil {
			externalMessages = append(externalMessages, m)
			continue
		}
		tx, err := s.searchTransactionNearBlock(ctx, *m.Destination, m.CreatedLt, tx.BlockID, false, depth+1)
		if err != nil {
			return core.Trace{}, err
		}
		child, err := s.recursiveGetChildren(ctx, *tx, depth+1)
		if err != nil {
			return core.Trace{}, err
		}
		trace.Children = append(trace.Children, &child)
	}
	var err error
	trace.AccountInterfaces, err = s.getAccountInterfaces(ctx, tx.Account)
	if err != nil {
		return core.Trace{}, nil
	}
	trace.OutMsgs = externalMessages
	return trace, nil
}

func (s *LiteStorage) findRoot(ctx context.Context, tx *core.Transaction, depth int) (*core.Transaction, error) {
	if tx == nil {
		return nil, fmt.Errorf("can't find root of nil transaction")
	}
	if tx.InMsg == nil || tx.InMsg.IsExternal() || tx.InMsg.IsEmission() {
		return tx, nil
	}
	var err error
	tx, err = s.searchTransactionNearBlock(ctx, *tx.InMsg.Source, tx.InMsg.CreatedLt, tx.BlockID, true, depth)
	if err != nil {
		return nil, err
	}

	return s.findRoot(ctx, tx, depth+1)
}

func (s *LiteStorage) searchTransactionNearBlock(ctx context.Context, a tongo.AccountID, lt uint64, blockID tongo.BlockID, back bool, depth int) (*core.Transaction, error) {
	if depth > maxDepthLimit {
		return nil, fmt.Errorf("can't find tx because of depth limit")
	}
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
	return tx, nil
}

func (s *LiteStorage) searchTransactionInBlock(ctx context.Context, a tongo.AccountID, lt uint64, blockID tongo.BlockID, back bool) (*core.Transaction, error) {
	blockIDExt, _, err := s.client.LookupBlock(ctx, blockID, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	block, prs := s.blockCache.Load(blockIDExt)
	if !prs {
		b, err := s.client.GetBlock(ctx, blockIDExt)
		if err != nil {
			return nil, err
		}
		s.blockCache.Store(blockIDExt, &b)
		block = &b
	}
	for _, tx := range block.AllTransactions() {
		if tx.AccountAddr != a.Address {
			continue
		}
		inMsg := tx.Msgs.InMsg
		if !back && inMsg.Exists && inMsg.Value.Value.Info.IntMsgInfo != nil && inMsg.Value.Value.Info.IntMsgInfo.CreatedLt == lt {
			return core.ConvertTransaction(a.Workchain, tongo.Transaction{BlockID: blockIDExt, Transaction: *tx})
		}
		if back {
			for _, m := range tx.Msgs.OutMsgs.Values() {
				if m.Value.Info.IntMsgInfo != nil && m.Value.Info.IntMsgInfo.CreatedLt == lt {
					return core.ConvertTransaction(a.Workchain, tongo.Transaction{BlockID: blockIDExt, Transaction: *tx})
				}
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *LiteStorage) getAccountInterfaces(ctx context.Context, id tongo.AccountID) ([]abi.ContractInterface, error) {
	interfaces, ok := s.accountInterfacesCache.Load(id)
	if ok {
		return interfaces, nil
	}
	account, err := s.GetRawAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	inspector := abi.NewContractInspector()
	cd, err := inspector.InspectContract(ctx, account.Code, s.executor, id)
	if err != nil {
		return nil, err
	}
	interfaces = cd.ImplementedInterfaces()
	s.accountInterfacesCache.Store(id, interfaces)
	return interfaces, nil
}
