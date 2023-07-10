package api

import (
	"context"
	"errors"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
)

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
	transaction := convertTransaction(*txs, h.addressBook)
	return &transaction, nil
}

func (h Handler) GetTransactionByMessageHash(ctx context.Context, params oas.GetTransactionByMessageHashParams) (r oas.GetTransactionByMessageHashRes, _ error) {
	hash, err := tongo.ParseHash(params.MsgID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	txs, err := h.storage.GetTransactionByMsg(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NotFound{Error: "transaction not found"}, nil
	}
	if err != nil {
		return nil, err
	}
	transaction := convertTransaction(*txs, h.addressBook)
	return &transaction, nil
}

func (h Handler) GetBlockTransactions(ctx context.Context, params oas.GetBlockTransactionsParams) (oas.GetBlockTransactionsRes, error) {
	id, err := blockIdFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	transactions, err := h.storage.GetBlockTransactions(ctx, id)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	res := oas.Transactions{
		Transactions: make([]oas.Transaction, 0, len(transactions)),
	}
	for _, tx := range transactions {
		res.Transactions = append(res.Transactions, convertTransaction(*tx, h.addressBook))
	}
	return &res, nil
}

func (h Handler) GetMasterchainHead(ctx context.Context) (r oas.GetMasterchainHeadRes, err error) {
	header, err := h.storage.LastMasterchainBlockHeader(ctx)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return g.Pointer(convertBlockHeader(*header)), nil
}

func (h Handler) GetConfig(ctx context.Context) (r oas.GetConfigRes, _ error) {
	cfg, err := h.storage.GetLastConfig()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	c := boc.NewCell()
	err = tlb.Marshal(c, cfg)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	raw, err := c.ToBocString()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	out, err := convertConfig(cfg)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	out.Raw = raw
	return out, nil
}
