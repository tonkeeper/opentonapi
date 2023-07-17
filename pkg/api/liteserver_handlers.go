package api

import (
	"context"
	"encoding/json"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteclient"
)

func (h Handler) GetMasterchainInfoLiteServer(ctx context.Context) (res oas.GetMasterchainInfoLiteServerRes, err error) {
	info, err := h.storage.GetMasterchainInfoRaw(ctx)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertMasterchainInfo(info)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetMasterchainInfoExtLiteServer(ctx context.Context, params oas.GetMasterchainInfoExtLiteServerParams) (oas.GetMasterchainInfoExtLiteServerRes, error) {
	info, err := h.storage.GetMasterchainInfoExtRaw(ctx, params.Mode)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertMasterchainInfoExt(info)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetTimeLiteServer(ctx context.Context) (res oas.GetTimeLiteServerRes, err error) {
	time, err := h.storage.GetTimeRaw(ctx)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &oas.GetTimeLiteServerOK{Time: time}, nil
}

func (h Handler) GetBlockLiteServer(ctx context.Context, params oas.GetBlockLiteServerParams) (res oas.GetBlockLiteServerRes, err error) {
	id, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	block, err := h.storage.GetBlockRaw(ctx, id)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	resp, err := convertBlock(block)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetStateLiteServer(ctx context.Context, params oas.GetStateLiteServerParams) (res oas.GetStateLiteServerRes, err error) {
	id, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	state, err := h.storage.GetStateRaw(ctx, id)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	resp, err := convertState(state)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetBlockHeaderLiteServer(ctx context.Context, params oas.GetBlockHeaderLiteServerParams) (res oas.GetBlockHeaderLiteServerRes, err error) {
	id, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	blockHeader, err := h.storage.GetBlockHeaderRaw(ctx, id, params.Mode)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	resp, err := convertBlockHeaderRaw(blockHeader)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) SendMessageLiteServer(ctx context.Context, request oas.SendMessageLiteServerReq) (res oas.SendMessageLiteServerRes, err error) {
	payload, err := json.Marshal(request.Body)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	code, err := h.storage.SendMessageRaw(ctx, payload)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	return &oas.SendMessageLiteServerOK{Code: code}, nil
}

func (h Handler) GetAccountStateLiteServer(ctx context.Context, params oas.GetAccountStateLiteServerParams) (res oas.GetAccountStateLiteServerRes, err error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	accountState, err := h.storage.GetAccountStateRaw(ctx, accountID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertAccountState(accountState)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetShardInfoLiteServer(ctx context.Context, params oas.GetShardInfoLiteServerParams) (res oas.GetShardInfoLiteServerRes, err error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	shardInfo, err := h.storage.GetShardInfoRaw(ctx, blockID, params.Workchain, params.Shard, params.Exact)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertShardInfo(shardInfo)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetAllShardsInfoLiteServer(ctx context.Context, params oas.GetAllShardsInfoLiteServerParams) (res oas.GetAllShardsInfoLiteServerRes, err error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	shardsAllInfo, err := h.storage.GetShardsAllInfo(ctx, blockID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertShardsAllInfo(shardsAllInfo)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetTransactionsLiteServer(ctx context.Context, params oas.GetTransactionsLiteServerParams) (oas.GetTransactionsLiteServerRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	hash, err := tongo.ParseHash(params.Hash)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	txs, err := h.storage.GetTransactionsRaw(ctx, params.Count, accountID, params.Lt, hash)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertTransactions(txs)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetListBlockTransactionsLiteServer(ctx context.Context, params oas.GetListBlockTransactionsLiteServerParams) (res oas.GetListBlockTransactionsLiteServerRes, err error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var after *liteclient.LiteServerTransactionId3C
	if params.AccountID.Value != "" && params.Lt.Value != 0 {
		accountID, err := tongo.ParseAccountID(params.AccountID.Value)
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
		after.Account = accountID.Address
		after.Lt = params.Lt.Value
	}
	listBlockTxs, err := h.storage.ListBlockTransactionsRaw(ctx, blockID, params.Mode, params.Count, after)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertListBlockTxs(listBlockTxs)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetBlockProofLiteServer(ctx context.Context, params oas.GetBlockProofLiteServerParams) (res oas.GetBlockProofLiteServerRes, err error) {
	knownBlockID, err := blockIdExtFromString(params.KnownBlock)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var targetBlockID *tongo.BlockIDExt
	if params.TargetBlock.Value != "" {
		blockID, err := blockIdExtFromString(params.TargetBlock.Value)
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
		targetBlockID = &blockID
	}
	blockProof, err := h.storage.GetBlockProofRaw(ctx, knownBlockID, targetBlockID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertBlockProof(blockProof)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetConfigAllLiteServer(ctx context.Context, params oas.GetConfigAllLiteServerParams) (res oas.GetConfigAllLiteServerRes, err error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	config, err := h.storage.GetConfigAllRaw(ctx, params.Mode, blockID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertAllConfig(config)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}

func (h Handler) GetShardBlockProofLiteServer(ctx context.Context, params oas.GetShardBlockProofLiteServerParams) (res oas.GetShardBlockProofLiteServerRes, err error) {
	blockID, err := blockIdExtFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	shardBlockProof, err := h.storage.GetShardBlockProofRaw(ctx, blockID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	resp, err := convertShardBlockProof(shardBlockProof)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return resp, nil
}
