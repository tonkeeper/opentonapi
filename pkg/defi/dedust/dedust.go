package dedust

import (
	"context"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

const ProviderID = "dedust"

type Source interface {
	DedustPools(ctx context.Context, contracts []tongo.AccountID) (map[tongo.AccountID]core.DedustPool, error)
}

type PoolAssetInfo struct {
	Asset0 core.Currency
	Asset1 core.Currency
}

var poolCodeHashes = map[string]bool{
	g.MustHex2Base64("778f0d3fe6482c50888970df5e787f40f3a4ab282170c035a5920877058c99d3"): true,
	g.MustHex2Base64("1275095b6da3911292406f4f4386f9e780099b854c6dee9ee2895ddce70927c1"): true,
	g.MustHex2Base64("c0f9d14fbc8e14f0d72cba2214165eee35836ab174130912baf9dbfa43ead562"): true,
}

func PoolAssetInfos(ctx context.Context, source Source, logger *zap.Logger, masterMetas []core.JettonMaster) (map[tongo.AccountID]PoolAssetInfo, []core.JettonMaster) {
	infos := make(map[tongo.AccountID]PoolAssetInfo)
	remaining := make([]core.JettonMaster, 0, len(masterMetas))
	poolIDs := make([]tongo.AccountID, 0, len(masterMetas))
	for _, master := range masterMetas {
		if !poolCodeHashes[master.CodeHash] {
			remaining = append(remaining, master)
			continue
		}
		poolIDs = append(poolIDs, master.Address)
	}
	if len(poolIDs) == 0 {
		return infos, remaining
	}
	pools, err := source.DedustPools(ctx, poolIDs)
	if err != nil {
		logger.Warn("failed to get DeDust pools for asset info", zap.Error(err))
		return infos, remaining
	}
	for poolID, pool := range pools {
		infos[poolID] = PoolAssetInfo{
			Asset0: pool.Asset0,
			Asset1: pool.Asset1,
		}
	}
	return infos, remaining
}
