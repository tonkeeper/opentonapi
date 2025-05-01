package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/ton"
)

func (h *Handler) GetMultisigAccount(ctx context.Context, params oas.GetMultisigAccountParams) (*oas.Multisig, error) {
	accountID, err := ton.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	multisig, err := h.storage.GetMultisigByID(ctx, accountID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	converted, err := h.convertMultisig(ctx, *multisig)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return converted, nil
}

func (h *Handler) GetMultisigOrder(ctx context.Context, params oas.GetMultisigOrderParams) (*oas.MultisigOrder, error) {
	orderID, err := ton.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	multisigOrder, err := h.storage.GetMultisigOrderByID(ctx, orderID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	converted, err := h.convertMultisigOrder(ctx, *multisigOrder)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &converted, nil

}
