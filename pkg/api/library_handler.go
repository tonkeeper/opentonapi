package api

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/ton"
	"net/http"
)

func (h *Handler) GetLibraryByHash(ctx context.Context, params oas.GetLibraryByHashParams) (*oas.BlockchainLibrary, error) {
	var hash ton.Bits256
	err := hash.FromHex(params.Hash)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	libs, err := h.storage.GetLibraries(ctx, []ton.Bits256{hash})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if lib, ok := libs[hash]; ok {
		s, err := lib.ToBocString()
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		return &oas.BlockchainLibrary{
			Data: s,
		}, nil
	}
	return nil, toError(http.StatusNotFound, core.ErrEntityNotFound)
}
