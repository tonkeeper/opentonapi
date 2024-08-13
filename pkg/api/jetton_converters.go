package api

import (
	"context"
	"errors"
	"fmt"
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
		Image:        meta.Image,
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
	return metadata
}

func (h *Handler) convertJettonHistory(ctx context.Context, account ton.AccountID, master *ton.AccountID, traceIDs []ton.Bits256, acceptLanguage oas.OptString) ([]oas.AccountEvent, int64, error) {
	var lastLT uint64
	events := make([]oas.AccountEvent, 0, len(traceIDs))
	for _, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			if errors.Is(err, core.ErrTraceIsTooLong) {
				// we ignore this for now, because we believe that this case is extremely rare.
				continue
			}
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
			Account:    convertAccountAddress(account, h.addressBook),
			Timestamp:  trace.Utime,
			IsScam:     false,
			Lt:         int64(trace.Lt),
			InProgress: trace.InProgress(),
			Extra:      result.Extra(account),
		}
		for _, action := range result.Actions {
			if action.Type != bath.JettonTransfer && action.Type != bath.JettonBurn && action.Type != bath.JettonMint {
				continue
			}
			if master != nil && ((action.JettonTransfer != nil && action.JettonTransfer.Jetton != *master) ||
				(action.JettonMint != nil && action.JettonMint.Jetton != *master) ||
				(action.JettonBurn != nil && action.JettonBurn.Jetton != *master)) {
				continue
			}
			if !action.IsSubject(account) {
				continue
			}
			convertedAction, err := h.convertAction(ctx, &account, action, acceptLanguage)
			if err != nil {
				return nil, 0, err
			}
			event.Actions = append(event.Actions, convertedAction)
		}
		event.IsScam = h.spamFilter.CheckActions(result.Actions, &account)
		if len(event.Actions) == 0 {
			continue
		}
		events = append(events, event)
		lastLT = trace.Lt
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
