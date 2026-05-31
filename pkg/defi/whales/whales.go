package whales

import (
	"context"
	"fmt"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/defi"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

type PoolsSource interface {
	GetParticipatingInWhalesPools(ctx context.Context, id ton.AccountID) ([]core.Nominator, error)
}

func Assets(ctx context.Context, source PoolsSource, logger *zap.Logger, accountID ton.AccountID) []defi.Asset {
	whalesPools, err := source.GetParticipatingInWhalesPools(ctx, accountID)
	if err != nil {
		logger.Warn("failed to get whales positions", zap.Error(err))
		return nil
	}

	provider, providerOk := defi.GetProvider("whales")
	assets := make([]defi.Asset, 0, len(whalesPools))
	for _, w := range whalesPools {
		poolInfo, ok := references.WhalesPools[w.Pool]
		if !ok {
			continue
		}

		pool := w.Pool
		asset := defi.Asset{
			Type:        defi.AssetTypeStaking,
			PoolAddress: &pool,
			LockedAsset: defi.LockedAsset{
				Type:   defi.LockedAssetTypeNative,
				Amount: *big.NewInt(w.MemberBalance),
			},
		}
		if providerOk {
			p := provider
			p.Name = fmt.Sprintf("%s %s", poolInfo.Name, poolInfo.Queue)
			p.Description = fmt.Sprintf("TON Whales staking pool, %s", poolInfo.Queue)
			asset.Provider = p
		}
		assets = append(assets, asset)
	}
	return assets
}
