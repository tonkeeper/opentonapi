package api

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/spam"
	"github.com/tonkeeper/tongo"
)

func (h Handler) SendMessage(ctx context.Context, req oas.OptSendMessageReq) (r oas.SendMessageRes, _ error) {
	payload, err := base64.StdEncoding.DecodeString(req.Value.Boc)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if err := h.msgSender.SendMessage(ctx, payload); err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &oas.SendMessageOK{}, nil
}

func (h Handler) GetTrace(ctx context.Context, params oas.GetTraceParams) (r oas.GetTraceRes, _ error) {
	hash, err := tongo.ParseHash(params.TraceID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	t, err := h.storage.GetTrace(ctx, hash)
	if err != nil {
		return nil, err
	}
	trace := convertTrace(*t, h.addressBook)
	return &trace, nil
}

func (h Handler) GetEvent(ctx context.Context, params oas.GetEventParams) (oas.GetEventRes, error) {
	traceID, err := tongo.ParseHash(params.EventID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	trace, err := h.storage.GetTrace(ctx, traceID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NotFound{Error: err.Error()}, nil
	}
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	b := bath.FromTrace(trace)
	bath.MergeAllBubbles(b, bath.DefaultStraws)
	actions, fees := bath.CollectActions(b, nil)
	event := oas.Event{
		EventID:    trace.Hash.Hex(),
		Timestamp:  trace.Utime,
		Actions:    make([]oas.Action, len(actions)),
		Fees:       make([]oas.Fee, len(fees)),
		IsScam:     false,
		Lt:         int64(trace.Lt),
		InProgress: trace.InProgress(),
	}
	for i, a := range actions {
		event.Actions[i] = convertAction(a, h.addressBook)
	}
	for i, f := range fees {
		event.Fees[i] = convertFees(f, h.addressBook)
	}
	return &event, nil
}

func (h Handler) GetEventsByAccount(ctx context.Context, params oas.GetEventsByAccountParams) (r oas.GetEventsByAccountRes, _ error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	traceIDs, err := h.storage.SearchTraces(ctx, account, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	events := make([]oas.AccountEvent, len(traceIDs))
	var lastLT uint64
	for i, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		b := bath.FromTrace(trace)
		bath.MergeAllBubbles(b, bath.DefaultStraws)
		actions, fees := bath.CollectActions(b, &account)
		e := oas.AccountEvent{
			EventID:    trace.Hash.Hex(),
			Account:    convertAccountAddress(account, h.addressBook),
			Timestamp:  trace.Utime,
			Fee:        oas.Fee{Account: convertAccountAddress(account, h.addressBook)},
			IsScam:     false,
			Lt:         int64(trace.Lt),
			InProgress: trace.InProgress(),
		}
		for _, f := range fees {
			if f.WhoPay == account {
				e.Fee = convertFees(f, h.addressBook)
				break
			}
		}
		for _, a := range actions {
			e.Actions = append(e.Actions, convertAction(a, h.addressBook))
		}
		if len(e.Actions) == 0 {
			e.Actions = []oas.Action{{
				Type:   oas.ActionTypeUnknown,
				Status: oas.ActionStatusOk,
			}}
		}
		e = spam.MarkScamEvent(e, h.spamWorker.GetRules())
		events[i] = e
		lastLT = trace.Lt
	}
	return &oas.AccountEvents{Events: events, NextFrom: int64(lastLT)}, nil
}

func optIntToPointer(o oas.OptInt64) *int64 {
	if !o.IsSet() {
		return nil
	}
	return &o.Value
}
