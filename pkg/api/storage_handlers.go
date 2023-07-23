package api

import (
	"context"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h Handler) GetStorageProviders(ctx context.Context) (*oas.GetStorageProvidersOK, error) {
	providers, err := h.storage.GetStorageProviders(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := make([]oas.StorageProvider, 0, len(providers))
	for _, p := range providers {
		result = append(result, oas.StorageProvider{
			Address:            p.Address.ToRaw(),
			AcceptNewContracts: p.AcceptNewContracts,
			RatePerMBDay:       p.RatePerMbDay,
			MaxSpan:            p.MaxSpan,
			MinimalFileSize:    p.MinimalFileSize,
			MaximalFileSize:    p.MaximalFileSize,
		})
	}
	return &oas.GetStorageProvidersOK{Providers: result}, nil
}
