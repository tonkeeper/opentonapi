package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

func (h *Handler) GetBlockchainBlock(ctx context.Context, params oas.GetBlockchainBlockParams) (*oas.BlockchainBlock, error) {
	blockID, err := ton.ParseBlockID(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	block, err := h.storage.GetBlockHeader(ctx, blockID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, err
	}
	res := convertBlockHeader(*block)
	return &res, nil
}

func (h *Handler) GetBlockchainBlockShards(ctx context.Context, params oas.GetBlockchainBlockShardsParams) (r *oas.BlockchainBlockShards, _ error) {
	blockID, err := ton.ParseBlockID(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	shards, err := h.storage.GetBlockShards(ctx, blockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := oas.BlockchainBlockShards{
		Shards: make([]oas.BlockchainBlockShardsShardsItem, len(shards)),
	}
	for i, shard := range shards {
		res.Shards[i] = oas.BlockchainBlockShardsShardsItem{
			LastKnownBlockID: shard.String(),
		}
	}
	return &res, nil
}

func (h *Handler) GetBlockchainBlockTransactions(ctx context.Context, params oas.GetBlockchainBlockTransactionsParams) (*oas.Transactions, error) {
	blockID, err := ton.ParseBlockID(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	transactions, err := h.storage.GetBlockTransactions(ctx, blockID)
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

func (h *Handler) GetBlockchainTransaction(ctx context.Context, params oas.GetBlockchainTransactionParams) (*oas.Transaction, error) {
	hash, err := tongo.ParseHash(params.TransactionID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	txs, err := h.storage.GetTransaction(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		txHash, err := h.storage.SearchTransactionByMessageHash(ctx, hash)
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		txs, err = h.storage.GetTransaction(ctx, *txHash)
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	transaction := convertTransaction(*txs, h.addressBook)
	return &transaction, nil
}

func (h *Handler) GetBlockchainTransactionByMessageHash(ctx context.Context, params oas.GetBlockchainTransactionByMessageHashParams) (*oas.Transaction, error) {
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

func (h *Handler) GetBlockchainMasterchainHead(ctx context.Context) (*oas.BlockchainBlock, error) {
	header, err := h.storage.LastMasterchainBlockHeader(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return g.Pointer(convertBlockHeader(*header)), nil
}

func (h *Handler) GetBlockchainConfig(ctx context.Context) (*oas.BlockchainConfig, error) {
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
	out, err := convertConfig(h.logger, cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	out.Raw = raw
	return out, nil
}

func convertConfigToOasConfig(conf *liteapi.BlockchainConfig) (*oas.RawBlockchainConfig, error) {
	// TODO: optimize this workflow
	value, err := json.Marshal(*conf)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	if err := json.Unmarshal(g.ChangeJsonKeys(value, g.CamelToSnake), &m); err != nil {
		return nil, err
	}
	return &oas.RawBlockchainConfig{Config: anyToJSONRawMap(m)}, nil
}

func (h *Handler) GetRawBlockchainConfig(ctx context.Context) (r *oas.RawBlockchainConfig, _ error) {
	cfg, err := h.storage.GetLastConfig(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	config, err := liteapi.ConvertBlockchainConfig(cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	rawConfig, err := convertConfigToOasConfig(config)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return rawConfig, nil
}

func (h *Handler) GetRawBlockchainConfigFromBlock(ctx context.Context, params oas.GetRawBlockchainConfigFromBlockParams) (r *oas.RawBlockchainConfig, _ error) {
	blockID, err := ton.ParseBlockID(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	cfg, err := h.storage.GetConfigFromBlock(ctx, blockID)
	if err != nil && errors.Is(err, core.ErrNotKeyBlock) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	config, err := liteapi.ConvertBlockchainConfig(cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	rawConfig, err := convertConfigToOasConfig(config)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return rawConfig, nil
}

func (h *Handler) GetBlockchainValidators(ctx context.Context) (*oas.Validators, error) {
	mcInfoExtra, err := h.storage.GetMasterchainInfoExtRaw(ctx, 0)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	blockHeader, err := h.storage.GetBlockHeader(ctx, ton.BlockID{
		Workchain: int32(mcInfoExtra.Last.Workchain),
		Shard:     mcInfoExtra.Last.Shard,
		Seqno:     mcInfoExtra.Last.Seqno,
	})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	fmt.Println(blockHeader, blockHeader.IsKeyBlock)
	if blockHeader.IsKeyBlock {
		blockConfig, err := h.storage.GetConfigFromBlock(ctx, blockHeader.BlockID)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		for key, value := range blockConfig.Config.Items() {
			fmt.Println(key, value)
		}
	}

	return &oas.Validators{}, nil
}
