package api

import (
	"context"
	"encoding/json"
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

func jettonPreview(master ton.AccountID, meta NormalizedMetadata, score int32, scaledUiParams *core.ScaledUIParameters) oas.JettonPreview {
	preview := oas.JettonPreview{
		Address:      master.ToRaw(),
		Name:         meta.Name,
		Symbol:       meta.Symbol,
		Verification: oas.JettonVerificationType(meta.Verification),
		Decimals:     meta.Decimals,
		Image:        meta.PreviewImage,
		Score:        score,
	}
	if meta.CustomPayloadApiUri != "" {
		preview.CustomPayloadAPIURI = oas.NewOptString(meta.CustomPayloadApiUri)
	}
	if scaledUiParams != nil {
		preview.ScaledUI.SetTo(oas.ScaledUI{
			Numerator:   scaledUiParams.Numerator.String(),
			Denominator: scaledUiParams.Denominator.String(),
		})
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
			convertedAction, err := h.convertAction(ctx, &account, action, acceptLanguage, event.Lt)
			if err != nil {
				return nil, 0, err
			}
			event.Actions = append(event.Actions, convertedAction)
		}
		event.IsScam = h.spamFilter.IsScamEvent(event.Actions, &account, trace.Account)
		if len(event.Actions) == 0 {
			continue
		}
		events = append(events, event)
		lastLT = trace.Lt
	}

	return events, int64(lastLT), nil
}

func (h *Handler) convertJettonOperation(ctx context.Context, op core.JettonOperation) (oas.JettonOperation, error) {
	b, err := json.Marshal(op.ForwardPayload)
	if err != nil {
		b = []byte{'{', '}'}
	}
	lt := int64(op.Lt)
	scaledUiParams, err := h.storage.GetScaledUIParameters(ctx, op.JettonMaster, &lt)
	if err != nil {
		return oas.JettonOperation{}, fmt.Errorf("failed to get scaled ui parameters: %w", err)
	}
	operation := oas.JettonOperation{
		Operation:       oas.JettonOperationOperation(op.Operation),
		Utime:           op.Utime,
		Lt:              lt,
		TransactionHash: op.TxID.Hex(),
		TraceID:         op.TraceID.Hex(),
		Source:          convertOptAccountAddress(op.Source, h.addressBook),
		Destination:     convertOptAccountAddress(op.Destination, h.addressBook),
		Jetton:          jettonPreview(op.JettonMaster, h.GetJettonNormalizedMetadata(ctx, op.JettonMaster), 0, scaledUiParams),
		Amount:          op.Amount.String(),
		Payload:         b,
	}
	return operation, nil
}

func (h *Handler) convertJettonBalance(ctx context.Context, wallet core.JettonWallet, currencies []string, scaledUiLt *int64) (oas.JettonBalance, error) {
	// the latest scaled ui parameters for jetton master if scaledUiLt == nil
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
	scaledUiParams, err := h.storage.GetScaledUIParameters(ctx, wallet.JettonAddress, scaledUiLt)
	if err != nil {
		return oas.JettonBalance{}, toError(http.StatusInternalServerError, err)
	}
	if wallet.Lock != nil {
		jettonBalance.Lock = oas.NewOptJettonBalanceLock(oas.JettonBalanceLock{
			Amount: wallet.Lock.FullBalance.String(),
			Till:   wallet.Lock.UnlockTime,
		})
	}
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
		normalizedMetadata = NormalizeMetadata(wallet.JettonAddress, meta, &info, core.TrustNone)
	} else {
		trust := core.TrustNone
		if h.spamFilter != nil {
			trust = h.spamFilter.JettonTrust(wallet.JettonAddress, meta.Symbol, meta.Name, meta.Image)
		}
		normalizedMetadata = NormalizeMetadata(wallet.JettonAddress, meta, nil, trust)
	}
	score, _ := h.score.GetJettonScore(wallet.JettonAddress)
	jettonBalance.Jetton = jettonPreview(wallet.JettonAddress, normalizedMetadata, score, scaledUiParams)

	return jettonBalance, nil
}

func (h *Handler) convertJettonInfo(ctx context.Context, master core.JettonMaster, holders map[tongo.AccountID]int32, scaledUiParams *core.ScaledUIParameters) oas.JettonInfo {
	meta := h.GetJettonNormalizedMetadata(ctx, master.Address)
	metadata := jettonMetadata(master.Address, meta)
	info := oas.JettonInfo{
		Mintable:     master.Mintable,
		TotalSupply:  master.TotalSupply.String(),
		Metadata:     metadata,
		Verification: oas.JettonVerificationType(meta.Verification),
		HoldersCount: holders[master.Address],
		Admin:        convertOptAccountAddress(master.Admin, h.addressBook),
		Preview:      meta.PreviewImage,
	}
	if scaledUiParams != nil {
		info.ScaledUI.SetTo(oas.ScaledUI{
			Numerator:   scaledUiParams.Numerator.String(),
			Denominator: scaledUiParams.Denominator.String(),
		})
	}
	return info
}
