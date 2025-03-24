package api

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/references"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h *Handler) GetExtraCurrencyInfo(ctx context.Context, params oas.GetExtraCurrencyInfoParams) (*oas.EcPreview, error) {
	meta := references.GetExtraCurrencyMeta(params.ID)
	return &oas.EcPreview{
		ID:       params.ID,
		Symbol:   meta.Symbol,
		Decimals: meta.Decimals,
		Image:    meta.Image,
	}, nil
}
