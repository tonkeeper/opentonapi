package api

import (
	"context"

	"github.com/go-faster/errors"

	"github.com/tonkeeper/opentonapi/internal/g"

	"github.com/tonkeeper/tongo"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods
	storage                  storage
	state                    chainState
}

func NewHandler(s storage, state chainState) Handler {
	return Handler{
		storage: s,
		state:   state,
	}
}

func (h Handler) GetAccount(ctx context.Context, params oas.GetAccountParams) (oas.GetAccountRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	info, err := h.storage.GetAccountInfo(ctx, accountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	res := convertToAccount(info)
	return &res, nil
}

func (h Handler) GetRawAccount(ctx context.Context, params oas.GetRawAccountParams) (r oas.GetRawAccountRes, _ error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, accountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	res := convertToRawAccount(rawAccount)
	return &res, nil
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
		result.Pools = append(result.Pools, oas.AccountStakingInfo{
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
	if w, prs := references.WhalesPools[poolID]; prs {
		poolConfig, poolStatus, err := h.storage.GetWhalesPoolInfo(ctx, poolID)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		return g.Pointer(convertStackingWhalesPool(poolID, w, poolStatus, poolConfig, h.state.GetAPY())), nil
	}

	return &oas.NotFound{Error: "pool not found"}, nil
}

func (h Handler) StackingPools(ctx context.Context, params oas.StackingPoolsParams) (r oas.StackingPoolsRes, _ error) {
	var result oas.StackingPoolsOK
	for k, w := range references.WhalesPools {
		poolConfig, poolStatus, err := h.storage.GetWhalesPoolInfo(ctx, k)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		result.Pools = append(result.Pools, convertStackingWhalesPool(k, w, poolStatus, poolConfig, h.state.GetAPY()))
	}
	return &result, nil
}

func (h Handler) GetNftItemsByAddresses(ctx context.Context, params oas.GetNftItemsByAddressesParams) (oas.GetNftItemsByAddressesRes, error) {
	accounts := make([]tongo.AccountID, len(params.AccountIds))
	var err error
	for i := range params.AccountIds {
		accounts[i], err = tongo.ParseAccountID(params.AccountIds[i])
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
	}
	items, err := h.storage.GetNFTs(ctx, accounts)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var result oas.NftItems
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(i))
	}
	return &result, nil
}
