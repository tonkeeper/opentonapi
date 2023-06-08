package api

import (
	"context"
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core/jetton"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func jettonPreview(master tongo.AccountID, meta jetton.NormalizedMetadata, imgGenerator previewGenerator) oas.JettonPreview {
	preview := oas.JettonPreview{
		Address:      master.ToRaw(),
		Name:         meta.Name,
		Symbol:       meta.Symbol,
		Verification: oas.JettonVerificationType(meta.Verification),
		Decimals:     meta.Decimals,
		Image:        imgGenerator.GenerateImageUrl(meta.Image, 200, 200),
	}
	return preview
}

func jettonMetadata(account tongo.AccountID, meta jetton.NormalizedMetadata) oas.JettonMetadata {
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
	return metadata
}

func (h Handler) convertJettonHistory(ctx context.Context, account tongo.AccountID, traceIDs []tongo.Bits256, acceptLanguage oas.OptString) ([]oas.AccountEvent, int64, error) {
	var lastLT uint64
	events := []oas.AccountEvent{}
	for _, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			return nil, 0, err
		}
		result, err := bath.FindActions(trace,
			bath.WithStraws([]bath.Straw{bath.FindJettonTransfer}),
			bath.WithMetaResolver(h))
		if err != nil {
			return nil, 0, err
		}
		event := oas.AccountEvent{
			EventID:    trace.Hash.Hex(),
			Account:    convertAccountAddress(account, h.addressBook),
			Timestamp:  trace.Utime,
			Fee:        oas.Fee{Account: convertAccountAddress(account, h.addressBook)},
			IsScam:     false,
			Lt:         int64(trace.Lt),
			InProgress: trace.InProgress(),
		}
		for _, action := range result.Actions {
			convertedAction, spamDetected := h.convertAction(ctx, action, acceptLanguage)
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
