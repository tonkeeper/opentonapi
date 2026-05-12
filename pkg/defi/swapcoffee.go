package defi

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

type swapCoffeeResponse struct {
	Pools []struct {
		PoolAddress string `json:"pool_address"`
		Liquidity   struct {
			UserAmount string `json:"user_amount"`
		} `json:"liquidity"`
	} `json:"pools"`
}

func collectSwapCoffeePositions(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	addr := account.ToHuman(true, false)
	url := fmt.Sprintf("https://backend.swap.coffee/v1/yield/pools?blockchains=ton&with_liquidity_from=%s", addr)

	var resp swapCoffeeResponse
	if err := fetchJSON(ctx, url, &resp); err != nil {
		d.warn("swap.coffee API error", zap.Error(err))
		return nil
	}

	var assets []oas.DefiAsset
	for _, p := range resp.Pools {
		amount, err := decimal.NewFromString(p.Liquidity.UserAmount)
		if err != nil || amount.IsZero() {
			continue
		}
		poolID, err := tongo.ParseAddress(p.PoolAddress)
		if err != nil {
			continue
		}
		meta := d.JettonMeta(ctx, poolID.ID)
		score, _ := d.Score.GetJettonScore(poolID.ID)
		asset := oas.DefiAsset{
			Type:     oas.DefiAssetTypeStaking,
			Provider: defiProvider(references.SwapCoffeeProvider),
			Amount:   amount.BigInt().Int64(),
			Position: JettonPositionNanoTON(rates, poolID.ID.ToRaw(), amount, meta.Decimals),
		}
		asset.Jetton.SetTo(JettonPreview(poolID.ID, meta, score))
		assets = append(assets, asset)
	}
	return assets
}
