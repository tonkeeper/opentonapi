package api

import (
	"context"
	"strconv"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
)

func convertJettonDecimals(decimals string) int {
	if decimals == "" {
		return 9
	}
	dec, err := strconv.Atoi(decimals)
	if err != nil {
		return 9
	}
	return dec
}

func jettonPreview(addressBook addressBook, master tongo.AccountID, meta tongo.JettonMetadata, imgGenerator previewGenerator) oas.JettonPreview {
	verification := oas.JettonVerificationTypeNone
	if meta.Name == "" {
		meta.Name = "Unknown Token"
	}
	if meta.Symbol == "" {
		meta.Symbol = "UKWN"
	}
	normalizedSymbol := strings.TrimSpace(strings.ToUpper(meta.Symbol))
	if normalizedSymbol == "TON" || normalizedSymbol == "TОN" { //eng and russian
		meta.Symbol = "SCAM"
	}

	if meta.Image == "" {
		meta.Image = references.Placeholder
	}
	info, ok := addressBook.GetJettonInfoByAddress(master)
	if ok {
		meta.Name = rewriteIfNotEmpty(meta.Name, info.Name)
		meta.Description = rewriteIfNotEmpty(meta.Description, info.Description)
		meta.Image = rewriteIfNotEmpty(meta.Image, info.Image)
		meta.Symbol = rewriteIfNotEmpty(meta.Symbol, info.Symbol)
		verification = oas.JettonVerificationTypeWhitelist
	}

	jetton := oas.JettonPreview{
		Address:      master.ToRaw(),
		Name:         meta.Name,
		Symbol:       meta.Symbol,
		Verification: verification,
		Decimals:     convertJettonDecimals(meta.Decimals),
		Image:        imgGenerator.GenerateImageUrl(meta.Image, 200, 200),
	}
	return jetton
}

func (h Handler) convertJettonHistory(ctx context.Context, account tongo.AccountID, traceIDs []tongo.Bits256) ([]oas.AccountEvent, int64, error) {
	var lastLT uint64
	events := []oas.AccountEvent{}
	for _, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			return nil, 0, err
		}
		bubble := bath.FromTrace(trace)
		bath.MergeAllBubbles(bubble, []bath.Straw{bath.FindJettonTransfer})
		actions, fees := bath.CollectActions(bubble, &account)
		event := oas.AccountEvent{
			EventID:    trace.Hash.Hex(),
			Account:    convertAccountAddress(account, h.addressBook),
			Timestamp:  trace.Utime,
			Fee:        oas.Fee{Account: convertAccountAddress(account, h.addressBook)},
			IsScam:     false,
			Lt:         int64(trace.Lt),
			InProgress: trace.InProgress(),
		}
		for _, fee := range fees {
			if fee.WhoPay == account {
				event.Fee = convertFees(fee, h.addressBook)
				break
			}
		}
		for _, action := range actions {
			convertedAction, spamDetected := h.convertAction(ctx, action)
			if !event.IsScam && spamDetected {
				event.IsScam = true
			}
			if convertedAction.Type != oas.ActionTypeJettonTransfer {
				continue
			}
			event.Actions = append(event.Actions, convertedAction)
		}
		if len(event.Actions) == 0 {
			continue
		}
		events = append(events, event)
		lastLT = trace.Lt
	}

	return events, int64(lastLT), nil
}
