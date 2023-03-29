package api

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
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
	trace := convertTrace(*t)
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
	actions, fees := bath.CollectActions(b)
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
		event.Actions[i] = convertAction(a)
	}
	for i, f := range fees {
		event.Fees[i] = convertFees(f)
	}
	return &event, nil
}
