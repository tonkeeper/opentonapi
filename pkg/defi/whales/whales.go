// Package whales resolves an account's TON Whales staking positions into
// defi.Asset values, independent of the API schema.
package whales

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/defi"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

// PoolsSource provides the whales pools an account participates in.
type PoolsSource interface {
	GetParticipatingInWhalesPools(ctx context.Context, id ton.AccountID) ([]core.Nominator, error)
}

// Assets returns the TON Whales staking positions of an account.
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
			Amount:      strconv.FormatInt(w.MemberBalance, 10),
			PoolAddress: &pool,
			LockedAsset: defi.LockedAsset{Type: defi.LockedAssetTypeNative},
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
