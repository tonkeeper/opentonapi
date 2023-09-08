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

func (h Handler) GetBlockchainBlock(ctx context.Context, params oas.GetBlockchainBlockParams) (*oas.BlockchainBlock, error) {
	id, err := blockIdFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	block, err := h.storage.GetBlockHeader(ctx, id)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, err
	}
	res := convertBlockHeader(*block)
	return &res, nil
}

func (h Handler) GetBlockchainBlockTransactions(ctx context.Context, params oas.GetBlockchainBlockTransactionsParams) (*oas.Transactions, error) {
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
		res.Transactions = append(res.Transactions, convertTransaction(*tx, h.addressBook))
	}
	return &res, nil
}

func (h Handler) GetBlockchainTransaction(ctx context.Context, params oas.GetBlockchainTransactionParams) (*oas.Transaction, error) {
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
	transaction := convertTransaction(*txs, h.addressBook)
	return &transaction, nil
}

func (h Handler) GetBlockchainTransactionByMessageHash(ctx context.Context, params oas.GetBlockchainTransactionByMessageHashParams) (*oas.Transaction, error) {
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
	transaction := convertTransaction(*txs, h.addressBook)
	return &transaction, nil
}

func (h Handler) GetBlockchainMasterchainHead(ctx context.Context) (*oas.BlockchainBlock, error) {
	header, err := h.storage.LastMasterchainBlockHeader(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return g.Pointer(convertBlockHeader(*header)), nil
}

func (h Handler) GetBlockchainConfig(ctx context.Context) (*oas.BlockchainConfig, error) {
	cfg, err := h.storage.GetLastConfig(ctx)
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
