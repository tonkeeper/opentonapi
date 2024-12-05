package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"net/http"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/ton"
)

func jettonPreview(master ton.AccountID, meta NormalizedMetadata) oas.JettonPreview {
	preview := oas.JettonPreview{
		Address:      master.ToRaw(),
		Name:         meta.Name,
		Symbol:       meta.Symbol,
		Verification: oas.JettonVerificationType(meta.Verification),
		Decimals:     meta.Decimals,
		Image:        meta.PreviewImage,
	}
	if meta.CustomPayloadApiUri != "" {
		preview.CustomPayloadAPIURI = oas.NewOptString(meta.CustomPayloadApiUri)
	}
	return preview
}

func jettonMetadata(account ton.AccountID, meta NormalizedMetadata) oas.JettonMetadata {
	metadata := oas.JettonMetadata{
		Address:  account.ToRaw(),
		Name:     meta.Name,
		Symbol:   meta.Symbol,
		Decimals: fmt.Sprintf("%d", meta.Decimals),
		Social:   meta.Social,
		Websites: meta.Websites,
	}
	if meta.Description != "" {
		metadata.Description.SetTo(meta.Description)
	}
	if meta.Image != "" {
		metadata.Image.SetTo(meta.Image)
	}
	if meta.CustomPayloadApiUri != "" {
		metadata.CustomPayloadAPIURI.SetTo(meta.CustomPayloadApiUri)
	}
	return metadata
}

func (h *Handler) convertJettonHistory(ctx context.Context, account ton.AccountID, master *ton.AccountID, history []core.JettonOperation, acceptLanguage oas.OptString) ([]oas.AccountEvent, int64, error) {
	var lastLT uint64
	var events []oas.AccountEvent
	res := make(map[core.TraceID]oas.AccountEvent)

	for _, op := range history {
		event, ok := res[op.TraceID]
		if !ok {
			event = oas.AccountEvent{
				EventID:   op.TraceID.Hash.Hex(),
				Account:   convertAccountAddress(account, h.addressBook),
				Timestamp: op.TraceID.UTime,
				IsScam:    false,
				Lt:        int64(op.TraceID.Lt),
				Extra:     0,
			}
		}

		var action bath.Action
		switch op.Operation {
		case core.TransferJettonOperation:
			transferAction := bath.JettonTransferAction{
				Jetton:    op.JettonMaster,
				Recipient: op.Destination,
				Sender:    op.Source,
				Amount:    tlb.VarUInteger16(*op.Amount.BigInt()),
			}
			action.Type = "JettonTransfer"
			action.JettonTransfer = &transferAction
			var payload abi.JettonPayload
			err := json.Unmarshal([]byte(op.ForwardPayload), &payload)
			if err != nil {
				break
			}
			switch p := payload.Value.(type) {
			case abi.TextCommentJettonPayload:
				comment := string(p.Text)
				action.JettonTransfer.Comment = &comment
			case abi.EncryptedTextCommentJettonPayload:
				action.JettonTransfer.EncryptedComment = &bath.EncryptedComment{EncryptionType: "simple", CipherText: p.CipherText}
			}
		case core.MintJettonOperation:
			mintAction := bath.JettonMintAction{
				Jetton:    op.JettonMaster,
				Recipient: *op.Destination,
				Amount:    tlb.VarUInteger16(*op.Amount.BigInt()),
			}
			action.Type = "JettonMint"
			action.JettonMint = &mintAction
		case core.BurnJettonOperation:
			burnAction := bath.JettonBurnAction{
				Jetton: op.JettonMaster,
				Sender: *op.Source,
				Amount: tlb.VarUInteger16(*op.Amount.BigInt()),
			}
			action.Type = "JettonTransfer"
			action.JettonBurn = &burnAction
		default:
			continue
		}
		convertedAction, err := h.convertAction(ctx, &account, action, acceptLanguage)
		if err != nil {
			return nil, 0, err
		}
		event.Actions = append(event.Actions, convertedAction)
		if op.Lt > lastLT {
			lastLT = op.Lt
		}
		res[op.TraceID] = event
	}

	for _, event := range res {
		event.IsScam = h.spamFilter.CheckActions(event.Actions, &account, nil)
		if len(event.Actions) == 0 {
			continue
		}
		events = append(events, event)
	}

	return events, int64(lastLT), nil
}

func (h *Handler) convertJettonBalance(ctx context.Context, wallet core.JettonWallet, currencies []string) (oas.JettonBalance, error) {
	todayRates, yesterdayRates, weekRates, monthRates, _ := h.getRates()
	for idx, currency := range currencies {
		if jetton, err := tongo.ParseAddress(currency); err == nil {
			currency = jetton.ID.ToRaw()
		} else {
			currency = strings.ToUpper(currency)
		}
		currencies[idx] = currency
	}
	jettonBalance := oas.JettonBalance{
		Balance:       wallet.Balance.String(),
		WalletAddress: convertAccountAddress(wallet.Address, h.addressBook),
		Extensions:    wallet.Extensions,
	}
	if wallet.Lock != nil {
		jettonBalance.Lock = oas.NewOptJettonBalanceLock(oas.JettonBalanceLock{
			Amount: wallet.Lock.FullBalance.String(),
			Till:   wallet.Lock.UnlockTime,
		})
	}
	var err error
	rates := make(map[string]oas.TokenRates)
	for _, currency := range currencies {
		rates, err = convertRates(rates, wallet.JettonAddress.ToRaw(), currency, todayRates, yesterdayRates, weekRates, monthRates)
		if err != nil {
			continue
		}
	}
	price := rates[wallet.JettonAddress.ToRaw()]
	if len(rates) > 0 && len(price.Prices.Value) > 0 {
		jettonBalance.Price.SetTo(price)
	}
	meta, err := h.storage.GetJettonMasterMetadata(ctx, wallet.JettonAddress)
	if err != nil && err.Error() == "not enough refs" {
		// happens when metadata is broken, for example.
		return oas.JettonBalance{}, toError(http.StatusInternalServerError, err)
	}
	if err != nil && errors.Is(err, liteapi.ErrOnchainContentOnly) {
		// we don't support such jettons
		return oas.JettonBalance{}, toError(http.StatusInternalServerError, err)
	}
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return oas.JettonBalance{}, toError(http.StatusNotFound, err)
	}
	var normalizedMetadata NormalizedMetadata
	info, ok := h.addressBook.GetJettonInfoByAddress(wallet.JettonAddress)
	if ok {
		normalizedMetadata = NormalizeMetadata(meta, &info, core.TrustNone)
	} else {
		trust := core.TrustNone
		if h.spamFilter != nil {
			trust = h.spamFilter.JettonTrust(wallet.JettonAddress, meta.Symbol, meta.Name, meta.Image)
		}
		normalizedMetadata = NormalizeMetadata(meta, nil, trust)
	}
	jettonBalance.Jetton = jettonPreview(wallet.JettonAddress, normalizedMetadata)

	return jettonBalance, nil
}

func (h *Handler) convertJettonInfo(ctx context.Context, master core.JettonMaster, holders map[tongo.AccountID]int32) oas.JettonInfo {
	meta := h.GetJettonNormalizedMetadata(ctx, master.Address)
	metadata := jettonMetadata(master.Address, meta)
	return oas.JettonInfo{
		Mintable:     master.Mintable,
		TotalSupply:  master.TotalSupply.String(),
		Metadata:     metadata,
		Verification: oas.JettonVerificationType(meta.Verification),
		HoldersCount: holders[master.Address],
		Admin:        convertOptAccountAddress(master.Admin, h.addressBook),
		Preview:      meta.PreviewImage,
	}
}
