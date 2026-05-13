package api

import (
	"context"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/defi"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func (h *Handler) defiDeps() defi.Deps {
	return defi.Deps{
		Storage:   h.storage,
		Executor:  h.executor,
		Score:     h.score,
		Logger:    h.logger,
		ProxyURL:  h.proxyURL,
		JettonMeta: func(ctx context.Context, m tongo.AccountID) defi.NormalizedJettonMeta {
			x := h.GetJettonNormalizedMetadata(ctx, m)
			return defi.NormalizedJettonMeta{
				Name:                x.Name,
				Description:         x.Description,
				Image:               x.Image,
				Symbol:              x.Symbol,
				Decimals:            x.Decimals,
				Verification:        x.Verification,
				Social:              x.Social,
				Websites:            x.Websites,
				CustomPayloadApiUri: x.CustomPayloadApiUri,
				PreviewImage:        x.PreviewImage,
			}
		},
	}
}

func (h *Handler) GetAccountDefiAssets(ctx context.Context, params oas.GetAccountDefiAssetsParams) (*oas.DefiAssets, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}

	todayRates, _, _, _, _ := h.getRates()
	return defi.GetDefiAssets(ctx, h.defiDeps(), account.ID, todayRates), nil
}
