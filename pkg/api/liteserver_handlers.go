package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteclient"
)

func (h *Handler) GetRawMasterchainInfo(ctx context.Context) (*oas.GetRawMasterchainInfoOK, error) {
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

func (h *Handler) GetRawMasterchainInfoExt(ctx context.Context, params oas.GetRawMasterchainInfoExtParams) (*oas.GetRawMasterchainInfoExtOK, error) {
	info, err := h.storage.GetMasterchainInfoExtRaw(ctx, uint32(params.Mode))
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	resp, err := convertMasterchainInfoExt(info)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h *Handler) GetRawTime(ctx context.Context) (*oas.GetRawTimeOK, error) {
	return &oas.GetRawTimeOK{Time: int32(time.Now().Unix())}, nil
}

func (h *Handler) GetRawBlockchainBlock(ctx context.Context, params oas.GetRawBlockchainBlockParams) (*oas.GetRawBlockchainBlockOK, error) {
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

func (h *Handler) GetRawBlockchainBlockState(ctx context.Context, params oas.GetRawBlockchainBlockStateParams) (*oas.GetRawBlockchainBlockStateOK, error) {
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

func (h *Handler) GetRawBlockchainBlockHeader(ctx context.Context, params oas.GetRawBlockchainBlockHeaderParams) (*oas.GetRawBlockchainBlockHeaderOK, error) {
	id, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	blockHeader, err := h.storage.GetBlockHeaderRaw(ctx, id, uint32(params.Mode))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertBlockHeaderRaw(blockHeader)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h *Handler) SendRawMessage(ctx context.Context, request *oas.SendRawMessageReq) (*oas.SendRawMessageOK, error) {
	payload, err := json.Marshal(request.Body)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	code, err := h.storage.SendMessageRaw(ctx, payload)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.SendRawMessageOK{Code: int32(code)}, nil
}

func (h *Handler) GetRawAccountState(ctx context.Context, params oas.GetRawAccountStateParams) (*oas.GetRawAccountStateOK, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var blockID *tongo.BlockIDExt
	if params.TargetBlock.IsSet() {
		id, err := blockIdExtFromString(params.TargetBlock.Value)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		blockID = &id
	}
	accountState, err := h.storage.GetAccountStateRaw(ctx, account.ID, blockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertAccountState(accountState)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h *Handler) GetRawShardInfo(ctx context.Context, params oas.GetRawShardInfoParams) (*oas.GetRawShardInfoOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	shardInfo, err := h.storage.GetShardInfoRaw(ctx, blockID, uint32(params.Workchain), uint64(params.Shard), params.Exact)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertShardInfo(shardInfo)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h *Handler) GetAllRawShardsInfo(ctx context.Context, params oas.GetAllRawShardsInfoParams) (*oas.GetAllRawShardsInfoOK, error) {
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

func (h *Handler) GetRawTransactions(ctx context.Context, params oas.GetRawTransactionsParams) (*oas.GetRawTransactionsOK, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	hash, err := tongo.ParseHash(params.Hash)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	txs, err := h.storage.GetTransactionsRaw(ctx, uint32(params.Count), account.ID, uint64(params.Lt), hash)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertTransactions(txs)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h *Handler) GetRawListBlockTransactions(ctx context.Context, params oas.GetRawListBlockTransactionsParams) (*oas.GetRawListBlockTransactionsOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var after *liteclient.LiteServerTransactionId3C
	if params.AccountID.Value != "" && params.Lt.Value != 0 {
		account, err := tongo.ParseAddress(params.AccountID.Value)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		after.Account = account.ID.Address
		after.Lt = uint64(params.Lt.Value)
	}
	listBlockTxs, err := h.storage.ListBlockTransactionsRaw(ctx, blockID, uint32(params.Mode), uint32(params.Count), after)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertListBlockTxs(listBlockTxs)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h *Handler) GetRawBlockProof(ctx context.Context, params oas.GetRawBlockProofParams) (*oas.GetRawBlockProofOK, error) {
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

func (h *Handler) GetRawConfig(ctx context.Context, params oas.GetRawConfigParams) (*oas.GetRawConfigOK, error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	config, err := h.storage.GetConfigAllRaw(ctx, uint32(params.Mode), blockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp, err := convertRawConfig(config)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return resp, nil
}

func (h *Handler) GetRawShardBlockProof(ctx context.Context, params oas.GetRawShardBlockProofParams) (*oas.GetRawShardBlockProofOK, error) {
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

func (h *Handler) GetOutMsgQueueSizes(ctx context.Context) (*oas.GetOutMsgQueueSizesOK, error) {
	outMsgQueueSizes, err := h.storage.GetOutMsgQueueSizes(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var convertedShards []oas.GetOutMsgQueueSizesOKShardsItem
	for _, shard := range outMsgQueueSizes.Shards {
		convertedShards = append(convertedShards, oas.GetOutMsgQueueSizesOKShardsItem{
			ID:   convertBlockIDRaw(shard.Id),
			Size: shard.Size,
		})
	}
	return &oas.GetOutMsgQueueSizesOK{
		ExtMsgQueueSizeLimit: outMsgQueueSizes.ExtMsgQueueSizeLimit,
		Shards:               convertedShards,
	}, nil
}
