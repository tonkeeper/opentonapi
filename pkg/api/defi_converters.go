package api

import (
	"fmt"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/defi"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

const defaultPublicAPIURL = "https://tonapi.io"

func (h *Handler) optJettonAssetInfo(info defi.AssetInfo, ok bool) *oas.JettonAssetInfo {
	if !ok {
		return nil
	}
	converted := h.convertJettonAssetInfo(info)
	return &converted
}

func (h *Handler) convertJettonAssetInfo(info defi.AssetInfo) oas.JettonAssetInfo {
	return oas.JettonAssetInfo{
		TokenType:    convertJettonAssetTokenType(info.TokenType),
		DefiProvider: h.convertJettonDefiProvider(info.DefiProvider),
	}
}

func convertJettonAssetTokenType(tokenType defi.TokenType) oas.JettonAssetTokenType {
	switch tokenType {
	case defi.TokenTypeLiquidStaking:
		return oas.JettonAssetTokenTypeLiquidStaking
	case defi.TokenTypeLiquidPool:
		return oas.JettonAssetTokenTypeLiquidPool
	case defi.TokenTypeYieldToken:
		return oas.JettonAssetTokenTypeYieldToken
	case defi.TokenTypeLendingSupply:
		return oas.JettonAssetTokenTypeLendingSupply
	default:
		panic(fmt.Sprintf("unknown defi token type: %q", tokenType))
	}
}

func (h *Handler) convertJettonDefiProvider(provider defi.Provider) oas.JettonDefiProvider {
	return oas.JettonDefiProvider{
		Name:        provider.Name,
		Description: provider.Description,
		Link:        provider.Link,
		MiniappLink: provider.MiniappLink,
		Icon:        h.absolutePublicURL(provider.Icon),
		Card:        h.absolutePublicURL(provider.Card),
		Full:        h.absolutePublicURL(provider.Full),
		Tag:         provider.Tag,
	}
}

func (h *Handler) absolutePublicURL(rawURL string) string {
	if rawURL == "" || strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	publicAPIURL := h.publicAPIURL
	if publicAPIURL == "" {
		publicAPIURL = defaultPublicAPIURL
	}
	return strings.TrimRight(publicAPIURL, "/") + "/" + strings.TrimLeft(rawURL, "/")
}
