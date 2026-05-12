package defi

import (
	"context"
	"math/big"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func collectLPPositions(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	wallets, err := d.Storage.GetJettonWalletsByOwnerAddress(ctx, account, nil, false, false, 1000, 0)
	if err != nil || len(wallets) == 0 {
		return nil
	}

	masters := make([]tongo.AccountID, 0, len(wallets))
	byMaster := make(map[tongo.AccountID]core.JettonWallet, len(wallets))
	for _, w := range wallets {
		if !w.Balance.IsZero() {
			masters = append(masters, w.JettonAddress)
			byMaster[w.JettonAddress] = w
		}
	}

	dedustPools, err := d.Storage.DedustPools(ctx, masters)
	if err != nil {
		return nil
	}

	var assets []oas.DefiAsset
	for poolAddr, pool := range dedustPools {
		w := byMaster[poolAddr]
		assets = append(assets, buildDedustLPAsset(ctx, d, poolAddr, w, pool, rates))
	}

	return assets
}

func buildDedustLPAsset(ctx context.Context, d Deps, poolAddr tongo.AccountID, w core.JettonWallet, pool core.DedustPool, rates map[string]float64) oas.DefiAsset {
	lpMeta := d.JettonMeta(ctx, poolAddr)
	score, _ := d.Score.GetJettonScore(poolAddr)
	pos := dedustLPPositionNanoTON(ctx, d, rates, poolAddr, pool, w.Balance)

	asset := oas.DefiAsset{
		Type:     oas.DefiAssetTypeLiquidPool,
		Provider: defiProvider(references.DedustProvider),
		Amount:   w.Balance.BigInt().Int64(),
		Position: pos,
	}
	asset.Jetton.SetTo(JettonPreview(poolAddr, lpMeta, score))
	return asset
}

func dedustLPPositionNanoTON(ctx context.Context, d Deps, rates map[string]float64, poolAddr tongo.AccountID, pool core.DedustPool, lpBalance decimal.Decimal) int64 {
	_, reservesRaw, err := abi.GetReserves(ctx, d.Executor, poolAddr)
	if err != nil {
		return 0
	}
	reserves, ok := reservesRaw.(abi.GetReserves_DedustResult)
	if !ok {
		return 0
	}

	masterData, err := d.Storage.GetJettonMasterData(ctx, poolAddr)
	if err != nil || masterData.TotalSupply.Sign() == 0 {
		return 0
	}

	r0 := new(big.Int).Set((*big.Int)(&reserves.Reserve0))
	r1 := new(big.Int).Set((*big.Int)(&reserves.Reserve1))

	c0 := currencyReserveToNanoTON(ctx, d, rates, r0, pool.Asset0)
	c1 := currencyReserveToNanoTON(ctx, d, rates, r1, pool.Asset1)

	tvl := c0 + c1
	if tvl == 0 {
		return 0
	}

	totalF, _ := new(big.Float).SetInt(&masterData.TotalSupply).Float64()
	balF, _ := lpBalance.Float64()
	if totalF == 0 {
		return 0
	}
	return int64(balF / totalF * tvl)
}
