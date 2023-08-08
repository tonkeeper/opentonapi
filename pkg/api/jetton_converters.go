package api

import (
	"context"
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func jettonPreview(master tongo.AccountID, meta NormalizedMetadata) oas.JettonPreview {
	preview := oas.JettonPreview{
		Address:      master.ToRaw(),
		Name:         meta.Name,
		Symbol:       meta.Symbol,
		Verification: oas.JettonVerificationType(meta.Verification),
		Decimals:     meta.Decimals,
		Image:        meta.Image,
	}
	return preview
}

func jettonMetadata(account tongo.AccountID, meta NormalizedMetadata) oas.JettonMetadata {
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

func (h Handler) convertJettonHistory(ctx context.Context, account tongo.AccountID, master *tongo.AccountID, traceIDs []tongo.Bits256, acceptLanguage oas.OptString) ([]oas.AccountEvent, int64, error) {
	var lastLT uint64
	events := []oas.AccountEvent{}
	for _, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			return nil, 0, err
		}
		result, err := bath.FindActions(ctx, trace,
			bath.WithStraws(bath.JettonTransfersBurnsMints),
			bath.WithInformationSource(h.storage))
		if err != nil {
			return nil, 0, err
		}
		event := oas.AccountEvent{
			EventID:    trace.Hash.Hex(),
			Account:    convertAccountAddress(account, h.addressBook, h.previewGenerator),
			Timestamp:  trace.Utime,
			IsScam:     false,
			Lt:         int64(trace.Lt),
			InProgress: trace.InProgress(),
		}
		for _, action := range result.Actions {
			if action.Type != bath.JettonTransfer {
				continue
			}
			if master != nil && action.JettonTransfer.Jetton != *master {
				continue
			}
			convertedAction, spamDetected, err := h.convertAction(ctx, account, action, acceptLanguage)
			if err != nil {
				return nil, 0, err
			}
			if !event.IsScam && spamDetected {
				event.IsScam = true
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
