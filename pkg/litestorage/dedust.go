package litestorage

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) DedustPools(ctx context.Context, poolIDs []tongo.AccountID) (map[tongo.AccountID]core.DedustPool, error) {
	pools := make(map[tongo.AccountID]core.DedustPool)
	for _, poolID := range poolIDs {
		_, value, err := abi.GetAssets(ctx, s.executor, poolID)
		if err != nil {
			return nil, err
		}
		if result, ok := value.(abi.GetAssets_DedustResult); ok {
			pools[poolID] = core.DedustPool{
				Asset0: DedustAssetToCurrency(result.Asset0),
				Asset1: DedustAssetToCurrency(result.Asset0),
			}
		}
	}
	return pools, nil
}

func DedustAssetToCurrency(asset abi.DedustAsset) core.Currency {
	switch asset.SumType {
	case "Native":
		return core.Currency{Type: core.CurrencyTON}
	case "Jetton":
		addr := tongo.NewAccountId(int32(asset.Jetton.WorkchainId), asset.Jetton.Address)
		return core.Currency{Type: core.CurrencyJetton, Jetton: addr}
	case "ExtraCurrency":
		return core.Currency{Type: core.CurrencyExtra, CurrencyID: &asset.ExtraCurrency.CurrencyId}
	}
	return core.Currency{}
}
