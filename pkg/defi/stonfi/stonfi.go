package stonfi

import (
	"context"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

const ProviderID = "stonfi"

type Source interface {
	JettonMastersForWallets(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]tongo.AccountID, error)
	STONfiPools(ctx context.Context, poolIDs []core.STONfiPoolID) (map[tongo.AccountID]core.STONfiPool, error)
}

type PoolAssetInfo struct {
	Asset0 core.Currency
	Asset1 core.Currency
}

var poolCodeHashVersions = map[string]core.STONfiVersion{
	g.MustHex2Base64("82566ad72b6568fe7276437d3b0c911aab65ed701c13601941b2917305e81c11"): core.STONfiPoolV1,
	g.MustHex2Base64("ec614ea4aaea3f7768606f1c1632b3374d3de096a1e7c4ba43c8009c487fee9d"): core.STONfiPoolV2,
	g.MustHex2Base64("f04a14c3231221056c3499965e4604417e324f8e9121d840120d803288715594"): core.STONfiPoolV2,
	g.MustHex2Base64("fbc7e8fcca72c2b9c078b359ffa936f46384491b895b6577b0a6cb3f569040bc"): core.STONfiPoolV2,
	g.MustHex2Base64("dac47636ae899081ebd4f47dc90ef9de98456b1000591069773f683c6d601fa9"): core.STONfiPoolV2,
	g.MustHex2Base64("cf5d0b99fa704e7cf2c9d50a8ff8b8bc7ce0b8a74e414b9c279ac544e7aade05"): core.STONfiPoolV2,
}

func isLiquidityPool(admin *tongo.AccountID, codeHash string) (core.STONfiVersion, bool) {
	if admin == nil {
		return "", false
	}
	if _, ok := references.StonfiWhitelistVaults[*admin]; !ok {
		return "", false
	}
	version, ok := poolCodeHashVersions[codeHash]
	if !ok {
		return "", false
	}
	return version, true
}

func PoolAssetInfos(ctx context.Context, source Source, logger *zap.Logger, masterMetas []core.JettonMaster) (map[tongo.AccountID]PoolAssetInfo, []core.JettonMaster) {
	infos := make(map[tongo.AccountID]PoolAssetInfo)
	remaining := make([]core.JettonMaster, 0, len(masterMetas))
	poolIDs := make([]core.STONfiPoolID, 0, len(masterMetas))
	for _, master := range masterMetas {
		version, ok := isLiquidityPool(master.Admin, master.CodeHash)
		if !ok {
			remaining = append(remaining, master)
			continue
		}
		poolIDs = append(poolIDs, core.STONfiPoolID{ID: master.Address, Version: version})
	}
	if len(poolIDs) == 0 {
		return infos, remaining
	}
	pools, err := source.STONfiPools(ctx, poolIDs)
	if err != nil {
		logger.Warn("failed to get STON.fi pools for asset info", zap.Error(err))
		return infos, remaining
	}
	wallets := make([]tongo.AccountID, 0, len(pools)*2)
	for _, pool := range pools {
		wallets = append(wallets, pool.Token0, pool.Token1)
	}
	masters, err := source.JettonMastersForWallets(ctx, wallets)
	if err != nil {
		logger.Warn("failed to get STON.fi pool jetton masters for asset info", zap.Error(err))
		return infos, remaining
	}
	for poolID, pool := range pools {
		master0, ok0 := masters[pool.Token0]
		master1, ok1 := masters[pool.Token1]
		if !(ok0 && ok1) {
			continue
		}
		infos[poolID] = PoolAssetInfo{
			Asset0: core.NewCurrencyJetton(&master0),
			Asset1: core.NewCurrencyJetton(&master1),
		}
	}
	return infos, remaining
}
