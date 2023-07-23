package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteclient"
)

func (h Handler) GetMasterchainInfoLiteServer(ctx context.Context) (*oas.GetMasterchainInfoLiteServerOK, error) {
	info, err := h.storage.GetMasterchainInfoRaw(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertMasterchainInfo(info)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetMasterchainInfoExtLiteServer(ctx context.Context, params oas.GetMasterchainInfoExtLiteServerParams) (*oas.GetMasterchainInfoExtLiteServerOK, error) {
	info, err := h.storage.GetMasterchainInfoExtRaw(ctx, params.Mode)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	resp, err := convertMasterchainInfoExt(info)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetTimeLiteServer(ctx context.Context) (*oas.GetTimeLiteServerOK, error) {
	time, err := h.storage.GetTimeRaw(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.GetTimeLiteServerOK{Time: time}, nil
}

func (h Handler) GetBlockLiteServer(ctx context.Context, params oas.GetBlockLiteServerParams) (*oas.GetBlockLiteServerOK, error) {
	id, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	block, err := h.storage.GetBlockRaw(ctx, id)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertBlock(block)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetStateLiteServer(ctx context.Context, params oas.GetStateLiteServerParams) (*oas.GetStateLiteServerOK, error) {
	id, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	state, err := h.storage.GetStateRaw(ctx, id)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertState(state)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetBlockHeaderLiteServer(ctx context.Context, params oas.GetBlockHeaderLiteServerParams) (*oas.GetBlockHeaderLiteServerOK, error) {
	id, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	blockHeader, err := h.storage.GetBlockHeaderRaw(ctx, id, params.Mode)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertBlockHeaderRaw(blockHeader)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) SendMessageLiteServer(ctx context.Context, request *oas.SendMessageLiteServerReq) (*oas.SendMessageLiteServerOK, error) {
	payload, err := json.Marshal(request.Body)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	code, err := h.storage.SendMessageRaw(ctx, payload)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.SendMessageLiteServerOK{Code: code}, nil
}

func (h Handler) GetAccountStateLiteServer(ctx context.Context, params oas.GetAccountStateLiteServerParams) (*oas.GetAccountStateLiteServerOK, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	accountState, err := h.storage.GetAccountStateRaw(ctx, accountID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertAccountState(accountState)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetShardInfoLiteServer(ctx context.Context, params oas.GetShardInfoLiteServerParams) (*oas.GetShardInfoLiteServerOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	shardInfo, err := h.storage.GetShardInfoRaw(ctx, blockID, params.Workchain, params.Shard, params.Exact)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertShardInfo(shardInfo)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetAllShardsInfoLiteServer(ctx context.Context, params oas.GetAllShardsInfoLiteServerParams) (*oas.GetAllShardsInfoLiteServerOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	shardsAllInfo, err := h.storage.GetShardsAllInfo(ctx, blockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertShardsAllInfo(shardsAllInfo)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetTransactionsLiteServer(ctx context.Context, params oas.GetTransactionsLiteServerParams) (*oas.GetTransactionsLiteServerOK, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	hash, err := tongo.ParseHash(params.Hash)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	txs, err := h.storage.GetTransactionsRaw(ctx, params.Count, accountID, params.Lt, hash)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertTransactions(txs)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetListBlockTransactionsLiteServer(ctx context.Context, params oas.GetListBlockTransactionsLiteServerParams) (*oas.GetListBlockTransactionsLiteServerOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var after *liteclient.LiteServerTransactionId3C
	if params.AccountID.Value != "" && params.Lt.Value != 0 {
		accountID, err := tongo.ParseAccountID(params.AccountID.Value)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		after.Account = accountID.Address
		after.Lt = params.Lt.Value
	}
	listBlockTxs, err := h.storage.ListBlockTransactionsRaw(ctx, blockID, params.Mode, params.Count, after)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertListBlockTxs(listBlockTxs)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetBlockProofLiteServer(ctx context.Context, params oas.GetBlockProofLiteServerParams) (*oas.GetBlockProofLiteServerOK, error) {
	knownBlockID, err := blockIdExtFromString(params.KnownBlock)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var targetBlockID *tongo.BlockIDExt
	if params.TargetBlock.Value != "" {
		blockID, err := blockIdExtFromString(params.TargetBlock.Value)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		targetBlockID = &blockID
	}
	blockProof, err := h.storage.GetBlockProofRaw(ctx, knownBlockID, targetBlockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertBlockProof(blockProof)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetConfigAllLiteServer(ctx context.Context, params oas.GetConfigAllLiteServerParams) (*oas.GetConfigAllLiteServerOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	config, err := h.storage.GetConfigAllRaw(ctx, params.Mode, blockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertRawConfig(config)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h Handler) GetShardBlockProofLiteServer(ctx context.Context, params oas.GetShardBlockProofLiteServerParams) (*oas.GetShardBlockProofLiteServerOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	shardBlockProof, err := h.storage.GetShardBlockProofRaw(ctx, blockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertShardBlockProof(shardBlockProof)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}
