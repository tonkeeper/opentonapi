package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
)

func (h Handler) GetBlock(ctx context.Context, params oas.GetBlockParams) (r *oas.Block, _ error) {
	id, err := blockIdFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	block, err := h.storage.GetBlockHeader(ctx, id)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return r, err
	}
	res := convertBlockHeader(*block)
	return &res, nil
}

func (h Handler) GetTransaction(ctx context.Context, params oas.GetTransactionParams) (r *oas.Transaction, _ error) {
	hash, err := tongo.ParseHash(params.TransactionID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	txs, err := h.storage.GetTransaction(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	transaction := convertTransaction(*txs, h.addressBook, h.previewGenerator)
	return &transaction, nil
}

func (h Handler) GetTransactionByMessageHash(ctx context.Context, params oas.GetTransactionByMessageHashParams) (*oas.Transaction, error) {
	hash, err := tongo.ParseHash(params.MsgID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	txHash, err := h.storage.SearchTransactionByMessageHash(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, fmt.Errorf("transaction not found"))
	} else if errors.Is(err, core.ErrTooManyEntities) {
		return nil, toError(http.StatusNotFound, fmt.Errorf("more than one transaction with messages hash"))
	}
	txs, err := h.storage.GetTransaction(ctx, *txHash)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	transaction := convertTransaction(*txs, h.addressBook, h.previewGenerator)
	return &transaction, nil
}

func (h Handler) GetBlockTransactions(ctx context.Context, params oas.GetBlockTransactionsParams) (*oas.Transactions, error) {
	id, err := blockIdFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	transactions, err := h.storage.GetBlockTransactions(ctx, id)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := oas.Transactions{
		Transactions: make([]oas.Transaction, 0, len(transactions)),
	}
	for _, tx := range transactions {
		res.Transactions = append(res.Transactions, convertTransaction(*tx, h.addressBook, h.previewGenerator))
	}
	return &res, nil
}

func (h Handler) GetMasterchainHead(ctx context.Context) (*oas.Block, error) {
	header, err := h.storage.LastMasterchainBlockHeader(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return g.Pointer(convertBlockHeader(*header)), nil
}

func (h Handler) GetConfig(ctx context.Context) (*oas.Config, error) {
	cfg, err := h.storage.GetLastConfig()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	c := boc.NewCell()
	err = tlb.Marshal(c, cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	raw, err := c.ToBocString()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	out, err := convertConfig(cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	out.Raw = raw
	return out, nil
}
