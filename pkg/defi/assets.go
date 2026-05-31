package defi

import (
	"context"
	_ "embed"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/defi/dedust"
	"github.com/tonkeeper/opentonapi/pkg/defi/stonfi"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

type Provider struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Link        string `json:"link"`
	MiniappLink string `json:"miniapp_link"`
	Icon        string `json:"icon"`
	Card        string `json:"card"`
	Full        string `json:"full"`
	Tag         string `json:"tag"`
}

type AssetInfo struct {
	TokenType     AssetType
	DefiProvider  Provider
	LiquidityPool *LiquidityPoolAssetInfo
}

type LiquidityPoolAssetInfo struct {
	Asset0 LockedAsset
	Asset1 LockedAsset
}

type jettonAssetDumpItem struct {
	TokenType AssetType `json:"token_type"`
	Provider  string    `json:"provider"`
}

type JettonAssetInfoSource interface {
	stonfi.Source
	dedust.Source

	GetJettonMastersByAddresses(ctx context.Context, addresses []ton.AccountID) ([]core.JettonMaster, error)
}

//go:embed defi_masters_mapping.json
var jettonAssetDump []byte

//go:embed defi_providers.json
var jettonProvidersDump []byte

var jettonAssetDumpInfos map[tongo.AccountID]AssetInfo
var jettonDefiProviders map[string]Provider
var stonfiProvider Provider
var dedustProvider Provider

func init() {
	jettonDefiProviders = g.MustParseJson[map[string]Provider](jettonProvidersDump)
	jettonAssetDumpInfos = parseJettonAssetDump(jettonAssetDump, jettonDefiProviders)
	stonfiProvider = g.MustGet(jettonDefiProviders, stonfi.ProviderID)
	dedustProvider = g.MustGet(jettonDefiProviders, dedust.ProviderID)
}

func parseJettonAssetDump(data []byte, providers map[string]Provider) map[tongo.AccountID]AssetInfo {
	raw := g.MustParseJson[map[tongo.AccountID]jettonAssetDumpItem](data)
	return g.MapMapValues(raw, func(item jettonAssetDumpItem) AssetInfo {
		provider, ok := providers[item.Provider]
		if !ok {
			panic("unknown defi provider: " + item.Provider)
		}
		if !validAssetType(item.TokenType) {
			panic("unknown defi token type: " + string(item.TokenType))
		}
		return AssetInfo{
			TokenType:    item.TokenType,
			DefiProvider: provider,
		}
	})
}

func validAssetType(tokenType AssetType) bool {
	switch tokenType {
	case AssetTypeLiquidStaking, AssetTypeLiquidPool, AssetTypeYieldToken, AssetTypeLendingSupply, AssetTypeLendingBorrow:
		return true
	default:
		return false
	}
}

func GetProvider(tag string) (Provider, bool) {
	p, ok := jettonDefiProviders[tag]
	return p, ok
}

func AssetInfos(ctx context.Context, source JettonAssetInfoSource, logger *zap.Logger, masters []tongo.AccountID) map[tongo.AccountID]AssetInfo {
	if len(masters) == 0 {
		return nil
	}

	result := make(map[tongo.AccountID]AssetInfo)
	unresolved := make([]tongo.AccountID, 0, len(masters))
	for _, master := range masters {
		if info, ok := jettonAssetDumpInfos[master]; ok {
			result[master] = info
		} else {
			unresolved = append(unresolved, master)
		}
	}
	if len(unresolved) == 0 {
		return result
	}

	masterMetas, err := source.GetJettonMastersByAddresses(ctx, unresolved)
	if err != nil {
		logger.Warn("failed to get jetton masters for asset info", zap.Error(err))
		return result
	}

	stonfiInfos, remainingJettons := stonfi.PoolAssetInfos(ctx, source, logger, masterMetas)
	for account, pool := range stonfiInfos {
		info := AssetInfo{
			TokenType: AssetTypeLiquidPool, DefiProvider: stonfiProvider,
			LiquidityPool: toPoolAssets(pool),
		}
		result[account] = info
	}

	dedustInfos, _ := dedust.PoolAssetInfos(ctx, source, logger, remainingJettons)
	for account, pool := range dedustInfos {
		info := AssetInfo{
			TokenType: AssetTypeLiquidPool, DefiProvider: dedustProvider,
			LiquidityPool: toPoolAssets(pool),
		}
		result[account] = info
	}

	return result
}

func toPoolAssets(pool struct {
	Asset0 core.Currency
	Asset1 core.Currency
}) *LiquidityPoolAssetInfo {
	return &LiquidityPoolAssetInfo{
		Asset0: currencyToLockedAsset(pool.Asset0),
		Asset1: currencyToLockedAsset(pool.Asset1),
	}
}

func currencyToLockedAsset(c core.Currency) LockedAsset {
	if c.Type == core.CurrencyJetton && c.Jetton != nil {
		return LockedAsset{Type: LockedAssetTypeJetton, JettonMaster: c.Jetton}
	}
	return LockedAsset{Type: LockedAssetTypeNative}
}
