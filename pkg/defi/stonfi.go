package defi

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

type stonfiPoolEntry struct {
	Address       string `json:"address"`
	Token0Address string `json:"token0_address"`
	Token1Address string `json:"token1_address"`
	Reserve0      string `json:"reserve0"`
	Reserve1      string `json:"reserve1"`
	LpTotalSupply string `json:"lp_total_supply"`
	LpBalance     string `json:"lp_balance"`
}

type stonfiWalletPoolsResponse struct {
	PoolList []stonfiPoolEntry `json:"pool_list"`
}

type stonfiWalletFarmsResponse struct {
	FarmList []struct {
		PoolAddress string `json:"pool_address"`
		NftInfos    []struct {
			StakedTokens string `json:"staked_tokens"`
			Status       string `json:"status"`
		} `json:"nft_infos"`
	} `json:"farm_list"`
}

type stonfiWalletStakesResponse struct {
	Nfts []struct {
		StakedTokens string `json:"staked_tokens"`
		Status       string `json:"status"`
	} `json:"nfts"`
}

func collectStonfiExternalPositions(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	addr := account.ToHuman(true, false)

	var (
		poolsResp stonfiWalletPoolsResponse
		farmsResp stonfiWalletFarmsResponse
		poolsErr  error
		farmsErr  error
		wg        sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		poolsErr = fetchJSON(ctx, fmt.Sprintf("https://api.ston.fi/v1/wallets/%s/pools", addr), &poolsResp)
	}()
	go func() {
		defer wg.Done()
		farmsErr = fetchJSON(ctx, fmt.Sprintf("https://api.ston.fi/v1/wallets/%s/farms", addr), &farmsResp)
	}()
	wg.Wait()

	var poolsByAddr map[string]stonfiPoolEntry
	if poolsErr == nil {
		poolsByAddr = stonfiPoolsByAddress(poolsResp)
	} else {
		d.warn("stonfi pools API error", zap.Error(poolsErr))
		poolsByAddr = make(map[string]stonfiPoolEntry)
	}

	var (
		lpAssets   []oas.DefiAsset
		farmAssets []oas.DefiAsset
		procWg     sync.WaitGroup
	)

	if poolsErr == nil {
		procWg.Add(1)
		go func() {
			defer procWg.Done()
			lpAssets = stonfiExternalLPAssets(ctx, d, poolsResp, rates)
		}()
	}

	if farmsErr != nil {
		d.warn("stonfi farms API error", zap.Error(farmsErr))
	} else {
		procWg.Add(1)
		go func() {
			defer procWg.Done()
			farmAssets = stonfiExternalFarmAssets(ctx, d, farmsResp, poolsByAddr, rates)
		}()
	}

	procWg.Wait()

	assets := make([]oas.DefiAsset, 0, len(lpAssets)+len(farmAssets))
	assets = append(assets, lpAssets...)
	assets = append(assets, farmAssets...)
	return assets
}

func stonfiPoolsByAddress(resp stonfiWalletPoolsResponse) map[string]stonfiPoolEntry {
	m := make(map[string]stonfiPoolEntry, len(resp.PoolList))
	for _, p := range resp.PoolList {
		m[p.Address] = p
	}
	return m
}

func stonfiExternalLPAssets(ctx context.Context, d Deps, poolsResp stonfiWalletPoolsResponse, rates map[string]float64) []oas.DefiAsset {
	var out []oas.DefiAsset
	for _, p := range poolsResp.PoolList {
		if p.LpBalance == "" || p.LpBalance == "0" {
			continue
		}
		lpBal, ok := new(big.Int).SetString(p.LpBalance, 10)
		if !ok || lpBal.Sign() == 0 {
			continue
		}
		poolID, err := tongo.ParseAddress(p.Address)
		if err != nil {
			continue
		}
		meta := d.JettonMeta(ctx, poolID.ID)
		score, _ := d.Score.GetJettonScore(poolID.ID)
		pos := stonfiPoolPositionNanoTON(ctx, d, rates, p.Reserve0, p.Reserve1, p.LpTotalSupply, p.Token0Address, p.Token1Address, lpBal)
		asset := oas.DefiAsset{
			Type:     oas.DefiAssetTypeLiquidPool,
			Provider: defiProvider(references.StonfiProvider),
			Amount:   lpBal.Int64(),
			Position: pos,
		}
		asset.Jetton.SetTo(JettonPreview(poolID.ID, meta, score))
		out = append(out, asset)
	}
	return out
}

func stonfiExternalFarmAssets(ctx context.Context, d Deps, farmsResp stonfiWalletFarmsResponse, poolsByAddr map[string]stonfiPoolEntry, rates map[string]float64) []oas.DefiAsset {
	var out []oas.DefiAsset
	for _, f := range farmsResp.FarmList {
		if len(f.NftInfos) == 0 {
			continue
		}
		pool, ok := poolsByAddr[f.PoolAddress]
		if !ok {
			continue
		}
		poolID, err := tongo.ParseAddress(f.PoolAddress)
		if err != nil {
			continue
		}
		meta := d.JettonMeta(ctx, poolID.ID)
		score, _ := d.Score.GetJettonScore(poolID.ID)
		for _, nft := range f.NftInfos {
			if nft.Status != "active" {
				continue
			}
			staked, ok := new(big.Int).SetString(nft.StakedTokens, 10)
			if !ok || staked.Sign() == 0 {
				continue
			}
			pos := stonfiPoolPositionNanoTON(ctx, d, rates, pool.Reserve0, pool.Reserve1, pool.LpTotalSupply, pool.Token0Address, pool.Token1Address, staked)
			asset := oas.DefiAsset{
				Type:     oas.DefiAssetTypeFarming,
				Provider: defiProvider(references.StonfiProvider),
				Amount:   staked.Int64(),
				Position: pos,
			}
			asset.Jetton.SetTo(JettonPreview(poolID.ID, meta, score))
			out = append(out, asset)
		}
	}
	return out
}

func collectStonfiStakingPositions(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	addr := account.ToHuman(true, false)
	var resp stonfiWalletStakesResponse
	if err := fetchJSON(ctx, fmt.Sprintf("https://api.ston.fi/v1/wallets/%s/stakes", addr), &resp); err != nil {
		d.warn("stonfi stakes API error", zap.Error(err))
		return nil
	}

	stonMaster := references.StonJettonMaster
	meta := d.JettonMeta(ctx, stonMaster)
	score, _ := d.Score.GetJettonScore(stonMaster)

	var assets []oas.DefiAsset
	for _, nft := range resp.Nfts {
		if nft.Status != "active" {
			continue
		}
		staked, ok := new(big.Int).SetString(nft.StakedTokens, 10)
		if !ok || staked.Sign() == 0 {
			continue
		}
		stakedDec := decimal.NewFromBigInt(staked, 0)
		asset := oas.DefiAsset{
			Type:     oas.DefiAssetTypeStaking,
			Provider: defiProvider(references.StonfiProvider),
			Amount:   staked.Int64(),
			Position: JettonPositionNanoTON(rates, stonMaster.ToRaw(), stakedDec, meta.Decimals),
		}
		asset.Jetton.SetTo(JettonPreview(stonMaster, meta, score))
		assets = append(assets, asset)
	}
	return assets
}

func stonfiPoolPositionNanoTON(ctx context.Context, d Deps, rates map[string]float64, reserve0Str, reserve1Str, totalSupplyStr, token0AddrStr, token1AddrStr string, lpBalance *big.Int) int64 {
	reserve0, ok0 := new(big.Int).SetString(reserve0Str, 10)
	reserve1, ok1 := new(big.Int).SetString(reserve1Str, 10)
	totalSupply, ok2 := new(big.Int).SetString(totalSupplyStr, 10)
	if !ok0 || !ok1 || !ok2 || totalSupply.Sign() == 0 {
		return 0
	}

	token0, err0 := tongo.ParseAddress(token0AddrStr)
	token1, err1 := tongo.ParseAddress(token1AddrStr)
	if err0 != nil || err1 != nil {
		return 0
	}

	a0 := core.Currency{Type: core.CurrencyJetton, Jetton: &token0.ID}
	a1 := core.Currency{Type: core.CurrencyJetton, Jetton: &token1.ID}
	c0 := currencyReserveToNanoTON(ctx, d, rates, reserve0, a0)
	c1 := currencyReserveToNanoTON(ctx, d, rates, reserve1, a1)
	tvl := c0 + c1
	if tvl == 0 {
		return 0
	}

	totalF, _ := new(big.Float).SetInt(totalSupply).Float64()
	balF, _ := new(big.Float).SetInt(lpBalance).Float64()
	return int64(balF / totalF * tvl)
}
