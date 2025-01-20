package api

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h *Handler) GetExtraCurrencyInfo(ctx context.Context, params oas.GetExtraCurrencyInfoParams) (*oas.EcPreview, error) {
	return &oas.EcPreview{}, nil
}
