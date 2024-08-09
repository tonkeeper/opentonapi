package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	tongoWallet "github.com/tonkeeper/tongo/wallet"
)

func convertMultisigActionsToRawMessages(actions []abi.MultisigSendMessageAction) ([]tongoWallet.RawMessage, error) {
	var messages []tongoWallet.RawMessage
	for _, action := range actions {
		switch action.SumType {
		case "SendMessage":
			msg := boc.NewCell()
			if err := tlb.Marshal(msg, action.SendMessage.Field0.Message); err != nil {
				return nil, err
			}
			messages = append(messages, tongoWallet.RawMessage{
				Message: msg,
				Mode:    action.SendMessage.Field0.Mode,
			})
		}
	}
	return messages, nil
}

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
