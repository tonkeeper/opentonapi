package api

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods
	storage                  storage
}

func NewHandler(s storage) Handler {
	return Handler{
		storage: s,
	}
}

func (h Handler) GetBlock(ctx context.Context, params oas.GetBlockParams) (r oas.GetBlockRes, _ error) {
	id, err := blockIdFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	block, err := h.storage.GetBlockHeader(ctx, id)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NotFound{Error: "block not found"}, nil
	}
	if err != nil {
		return r, err
	}
	res := convertBlockHeader(*block)
	return &res, nil
}

func (h Handler) GetTransaction(ctx context.Context, params oas.GetTransactionParams) (r oas.GetTransactionRes, _ error) {
	hash, err := tongo.ParseHash(params.TransactionID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	txs, err := h.storage.GetTransaction(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NotFound{Error: "transaction not found"}, nil
	}
	if err != nil {
		return nil, err
	}
	transaction := convertTransaction(*txs)
	return &transaction, nil
}

func (h Handler) GetTrace(ctx context.Context, params oas.GetTraceParams) (r oas.GetTraceRes, _ error) {
	hash, err := tongo.ParseHash(params.TraceID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	t, err := h.storage.GetTrace(ctx, hash)
	if err != nil {
		return nil, err
	}
	trace := convertTrace(*t)
	return &trace, nil
}

func (h Handler) PoolsByNominators(ctx context.Context, params oas.PoolsByNominatorsParams) (oas.PoolsByNominatorsRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	whalesPools, err := h.storage.GetParticipatingInWhalesPools(ctx, accountID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var result oas.AccountStacking
	for _, w := range whalesPools {
		if _, ok := references.WhalesPools[w.Pool]; !ok {
			continue //skip unknown pools
		}
		result.Whales = append(result.Whales, oas.AccountStakingInfo{
			Pool:            w.Pool.ToRaw(),
			Amount:          w.MemberBalance,
			PendingDeposit:  w.MemberPendingDeposit,
			PendingWithdraw: w.MemberPendingWithdraw,
		})
	}
	return &result, nil
}

func (h Handler) StackingPoolInfo(ctx context.Context, params oas.StackingPoolInfoParams) (oas.StackingPoolInfoRes, error) {
	poolID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var result oas.PoolInfo
	if w, prs := references.WhalesPools[poolID]; prs {
		result.SetAddress(poolID.ToRaw())
		result.SetImplementation(oas.PoolInfoImplementationWhales)
		result.SetApy(5.6)
		result.SetName(w.Name + " " + w.Queue)
		return &result, nil
	}

	return &oas.NotFound{Error: "pool not found"}, nil
}

func (h Handler) StackingPools(ctx context.Context, params oas.StackingPoolsParams) (r oas.StackingPoolsRes, _ error) {
	var result oas.StackingPoolsOK
	for k, w := range references.WhalesPools {
		result.Pools = append(result.Pools, oas.PoolInfo{
			Address:        k.ToRaw(),
			Name:           w.Name + " " + w.Queue,
			TotalAmount:    0,
			Implementation: oas.PoolInfoImplementationWhales,
			Apy:            5.6,
		})
	}
	return &result, nil
}
