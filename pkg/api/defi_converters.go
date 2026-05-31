package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/defi"
	"github.com/tonkeeper/opentonapi/pkg/defi/evaa"
	"github.com/tonkeeper/opentonapi/pkg/defi/whales"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

func (h *Handler) GetAccountDefiAssets(ctx context.Context, params oas.GetAccountDefiAssetsParams) (*oas.DefiAssets, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	positions := make([]defi.Asset, 0)
	positions = append(positions, whales.Assets(ctx, h.storage, h.logger, account.ID)...)
	positions = append(positions, evaa.Assets(ctx, h.executor, h.logger, account.ID)...)

	assets := make([]oas.DefiAsset, 0, len(positions))
	for _, position := range positions {
		assets = append(assets, h.convertDefiAsset(ctx, position))
	}
	return &oas.DefiAssets{Assets: assets}, nil
}

func (h *Handler) convertDefiAsset(ctx context.Context, asset defi.Asset) oas.DefiAsset {
	result := oas.DefiAsset{
		Type:         convertDefiAssetType(asset.Type),
		Amount:       asset.LockedAsset.Amount.String(),
		DefiProvider: h.convertDefiProvider(asset.Provider),
		LockedAsset:  h.convertDefiLockedAsset(ctx, asset.LockedAsset),
	}
	if asset.PoolAddress != nil {
		result.PoolAddress = oas.NewOptString(asset.PoolAddress.ToRaw())
	}
	if asset.AssetAddress != nil {
		result.AssetAddress = oas.NewOptString(asset.AssetAddress.ToRaw())
	}
	return result
}

var defi2oasMap = map[defi.AssetType]oas.DefiAssetType{
	defi.AssetTypeStaking:       oas.DefiAssetTypeStaking,
	defi.AssetTypeLiquidStaking: oas.DefiAssetTypeLiquidStaking,
	defi.AssetTypeLiquidPool:    oas.DefiAssetTypeLiquidPool,
	defi.AssetTypeYieldToken:    oas.DefiAssetTypeYieldToken,
	defi.AssetTypeLendingSupply: oas.DefiAssetTypeLendingSupply,
	defi.AssetTypeLendingBorrow: oas.DefiAssetTypeLendingBorrow,
}

func convertDefiAssetType(assetType defi.AssetType) oas.DefiAssetType {
	if oasType, ok := defi2oasMap[assetType]; ok {
		return oasType
	}
	return ""
}

func (h *Handler) convertDefiLockedAsset(ctx context.Context, locked defi.LockedAsset) oas.DefiLockedAsset {
	if locked.Type == defi.LockedAssetTypeJetton && locked.JettonMaster != nil {
		return oas.DefiLockedAsset{
			Type:   oas.DefiLockedAssetTypeJetton,
			Jetton: oas.NewOptJettonPreview(h.convertDefiJettonPreview(ctx, *locked.JettonMaster)),
		}
	}
	return oas.DefiLockedAsset{Type: oas.DefiLockedAssetTypeNative}
}

func (h *Handler) convertDefiJettonPreview(ctx context.Context, master ton.AccountID) oas.JettonPreview {
	meta := h.GetJettonNormalizedMetadata(ctx, master)
	var score int32
	if h.score != nil {
		score, _ = h.score.GetJettonScore(master)
	}
	return jettonPreview(master, meta, score, nil)
}

func (h *Handler) optJettonAssetInfo(ctx context.Context, info defi.AssetInfo, ok bool) *oas.JettonAssetInfo {
	if !ok {
		return nil
	}
	result := oas.JettonAssetInfo{
		TokenType:    convertDefiAssetType(info.TokenType),
		DefiProvider: h.convertDefiProvider(info.DefiProvider),
	}
	if info.LiquidityPool != nil {
		result.PoolAssets = oas.NewOptDefiLiquidPoolAssets(oas.DefiLiquidPoolAssets{
			Asset0: h.convertDefiLockedAsset(ctx, info.LiquidityPool.Asset0),
			Asset1: h.convertDefiLockedAsset(ctx, info.LiquidityPool.Asset1),
		})
	}
	converted := result
	return &converted
}

func (h *Handler) convertDefiProvider(provider defi.Provider) oas.DefiProvider {
	result := oas.DefiProvider{
		Name:        provider.Name,
		Description: provider.Description,
		Link:        provider.Link,
		Icon:        h.absolutePublicURL(provider.Icon),
		Card:        h.absolutePublicURL(provider.Card),
		Full:        h.absolutePublicURL(provider.Full),
		Tag:         provider.Tag,
	}
	if provider.MiniappLink != "" {
		result.MiniappLink = oas.NewOptString(provider.MiniappLink)
	}
	return result
}

func (h *Handler) absolutePublicURL(rawURL string) string {
	if rawURL == "" || strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	return strings.TrimRight(h.publicAPIURL, "/") + "/" + strings.TrimLeft(rawURL, "/")
}
