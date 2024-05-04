package litestorage

import (
	"context"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/liteclient"
)

func (s *LiteStorage) GetMasterchainInfoRaw(ctx context.Context) (liteclient.LiteServerMasterchainInfoC, error) {
	return s.client.GetMasterchainInfo(ctx)
}

func (s *LiteStorage) GetMasterchainInfoExtRaw(ctx context.Context, mode uint32) (liteclient.LiteServerMasterchainInfoExtC, error) {
	return s.client.GetMasterchainInfoExt(ctx, mode)
}

func (s *LiteStorage) GetTimeRaw(ctx context.Context) (uint32, error) {
	return s.client.GetTime(ctx)
}

func (s *LiteStorage) GetBlockRaw(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerBlockDataC, error) {
	return s.client.GetBlockRaw(ctx, id)
}

func (s *LiteStorage) GetStateRaw(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerBlockStateC, error) {
	return s.client.GetStateRaw(ctx, id)
}

func (s *LiteStorage) GetBlockHeaderRaw(ctx context.Context, id tongo.BlockIDExt, mode uint32) (liteclient.LiteServerBlockHeaderC, error) {
	return s.client.GetBlockHeaderRaw(ctx, id, mode)
}

func (s *LiteStorage) SendMessageRaw(ctx context.Context, payload []byte) (uint32, error) {
	return s.client.SendMessage(ctx, payload)
}

func (s *LiteStorage) GetAccountStateRaw(ctx context.Context, accountID tongo.AccountID, id *tongo.BlockIDExt) (liteclient.LiteServerAccountStateC, error) {
	if id != nil {
		return s.client.WithBlock(*id).GetAccountStateRaw(ctx, accountID)
	}
	return s.client.GetAccountStateRaw(ctx, accountID)
}

func (s *LiteStorage) GetShardInfoRaw(ctx context.Context, id tongo.BlockIDExt, workchain uint32, shard uint64, exact bool) (liteclient.LiteServerShardInfoC, error) {
	return s.client.GetShardInfoRaw(ctx, id, workchain, shard, exact)
}

func (s *LiteStorage) GetShardsAllInfo(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerAllShardsInfoC, error) {
	return s.client.GetAllShardsInfoRaw(ctx, id)
}

func (s *LiteStorage) GetTransactionsRaw(ctx context.Context, count uint32, accountID tongo.AccountID, lt uint64, hash tongo.Bits256) (liteclient.LiteServerTransactionListC, error) {
	return s.client.GetTransactionsRaw(ctx, count, accountID, lt, hash)
}

func (s *LiteStorage) ListBlockTransactionsRaw(ctx context.Context, id tongo.BlockIDExt, mode, count uint32, after *liteclient.LiteServerTransactionId3C) (liteclient.LiteServerBlockTransactionsC, error) {
	return s.client.ListBlockTransactionsRaw(ctx, id, mode, count, after)
}

func (s *LiteStorage) GetBlockProofRaw(ctx context.Context, knownBlock tongo.BlockIDExt, targetBlock *tongo.BlockIDExt) (liteclient.LiteServerPartialBlockProofC, error) {
	return s.client.GetBlockProofRaw(ctx, knownBlock, targetBlock)
}

func (s *LiteStorage) GetConfigAllRaw(ctx context.Context, mode uint32, id tongo.BlockIDExt) (liteclient.LiteServerConfigInfoC, error) {
	return s.client.WithBlock(id).GetConfigAllRaw(ctx, liteapi.ConfigMode(mode))
}

func (s *LiteStorage) GetShardBlockProofRaw(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerShardBlockProofC, error) {
	return s.client.WithBlock(id).GetShardBlockProofRaw(ctx)
}

func (s *LiteStorage) GetOutMsgQueueSizes(ctx context.Context) (liteclient.LiteServerOutMsgQueueSizesC, error) {
	return s.client.GetOutMsgQueueSizes(ctx)
}
