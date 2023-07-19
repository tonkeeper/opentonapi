package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/txemulator"
	"golang.org/x/exp/slices"
)

func (h Handler) SendMessage(ctx context.Context, req oas.SendMessageReq) (r oas.SendMessageRes, _ error) {
	if h.msgSender == nil {
		return nil, fmt.Errorf("msg sender is not configured")
	}
	payload, err := base64.StdEncoding.DecodeString(req.Boc)
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
	if hash.Hex() == testEventID {
		testTrace := getTestTrace()
		return &testTrace, nil
	}
	t, err := h.storage.GetTrace(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		txHash, err2 := h.storage.SearchTransactionByMessageHash(ctx, hash)
		if err2 != nil {
			return &oas.NotFound{Error: err.Error()}, nil
		}
		t, err = h.storage.GetTrace(ctx, *txHash)
	}
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
	if traceID.Hex() == testEventID {
		testEvent := getTestEvent()
		return &testEvent, nil
	}
	trace, err := h.storage.GetTrace(ctx, traceID)
	if errors.Is(err, core.ErrEntityNotFound) {
		txHash, err2 := h.storage.SearchTransactionByMessageHash(ctx, traceID)
		if err2 != nil {
			return &oas.NotFound{Error: err.Error()}, nil
		}
		trace, err = h.storage.GetTrace(ctx, *txHash)
	}
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	result, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	event, err := h.toEvent(ctx, trace, result, params.AcceptLanguage)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &event, nil
}

func (h Handler) GetEventsByAccount(ctx context.Context, params oas.GetEventsByAccountParams) (r oas.GetEventsByAccountRes, _ error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	traceIDs, err := h.storage.SearchTraces(ctx, account, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	events := make([]oas.AccountEvent, len(traceIDs))
	var lastLT uint64
	for i, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		result, err := bath.FindActions(ctx, trace, bath.ForAccount(account), bath.WithInformationSource(h.storage))
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		events[i], err = h.toAccountEvent(ctx, account, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		lastLT = trace.Lt
	}
	if account.ToRaw() == testEventAccount {
		events = slices.Insert(events, 0, getTestAccountEvent())
	}
	for _, event := range events {
		for i, j := 0, len(event.Actions)-1; i < j; i, j = i+1, j-1 {
			event.Actions[i], event.Actions[j] = event.Actions[j], event.Actions[i]
		}
	}
	return &oas.AccountEvents{Events: events, NextFrom: int64(lastLT)}, nil
}

func (h Handler) EmulateMessageToAccountEvent(ctx context.Context, req oas.EmulateMessageToAccountEventReq, params oas.EmulateMessageToAccountEventParams) (r oas.EmulateMessageToAccountEventRes, _ error) {
	c, err := boc.DeserializeSinglRootBase64(req.Boc)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{err.Error()}, nil
	}
	emulator, err := txemulator.NewTraceBuilder()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, err
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	trace, err := emulatedTreeToTrace(tree, emulator.FinalStates())
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	result, err := bath.FindActions(ctx, trace)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	event, err := h.toAccountEvent(ctx, account, trace, result, params.AcceptLanguage, false)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &event, nil
}

func (h Handler) EmulateMessageToEvent(ctx context.Context, req oas.EmulateMessageToEventReq, params oas.EmulateMessageToEventParams) (r oas.EmulateMessageToEventRes, _ error) {
	c, err := boc.DeserializeSinglRootBase64(req.Boc)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	emulator, err := txemulator.NewTraceBuilder()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, err
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	trace, err := emulatedTreeToTrace(tree, emulator.FinalStates())
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	result, err := bath.FindActions(ctx, trace)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	event, err := h.toEvent(ctx, trace, result, params.AcceptLanguage)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &event, nil
}

func (h Handler) EmulateMessageToTrace(ctx context.Context, req oas.EmulateMessageToTraceReq) (r oas.EmulateMessageToTraceRes, _ error) {
	c, err := boc.DeserializeSinglRootBase64(req.Boc)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	emulator, err := txemulator.NewTraceBuilder()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, err
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	trace, err := emulatedTreeToTrace(tree, emulator.FinalStates())
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	t := convertTrace(*trace, h.addressBook)
	return &t, nil
}

func extractDestinationWallet(message tlb.Message) (*tongo.AccountID, error) {
	if message.Info.SumType != "ExtInMsgInfo" {
		return nil, fmt.Errorf("unsupported message type: %v", message.Info.SumType)
	}
	accountID, err := tongo.AccountIDFromTlb(message.Info.ExtInMsgInfo.Dest)
	if err != nil {
		return nil, err
	}
	if accountID == nil {
		return nil, fmt.Errorf("failed to extract the destination wallet")
	}
	return accountID, nil
}

func (h Handler) EmulateWalletMessage(ctx context.Context, req oas.EmulateWalletMessageReq, params oas.EmulateWalletMessageParams) (oas.EmulateWalletMessageRes, error) {
	msgCell, err := boc.DeserializeSinglRootBase64(req.Boc)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var m tlb.Message
	err = tlb.Unmarshal(msgCell, &m)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	walletAddress, err := extractDestinationWallet(m)
	if err != nil {
		return &oas.BadRequest{err.Error()}, nil
	}
	account, err := h.storage.GetRawAccount(ctx, *walletAddress)
	if err != nil {
		// TODO: if not found, take code from stateInit
		return &oas.InternalError{Error: err.Error()}, nil
	}
	walletVersion, err := wallet.GetVersionByCode(account.Code)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	risk, err := wallet.ExtractRisk(walletVersion, msgCell)
	if err != nil {
		return nil, err
	}
	emulator, err := txemulator.NewTraceBuilder()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	trace, err := emulatedTreeToTrace(tree, emulator.FinalStates())
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	t := convertTrace(*trace, h.addressBook)
	result, err := bath.FindActions(ctx, trace, bath.ForAccount(*walletAddress))
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	event, err := h.toAccountEvent(ctx, *walletAddress, trace, result, params.AcceptLanguage, true)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	oasRisk, err := h.convertRisk(ctx, *risk, *walletAddress)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	consequences := oas.MessageConsequences{
		Trace: t,
		Event: event,
		Risk:  oasRisk,
	}
	return &consequences, nil
}

func emulatedTreeToTrace(tree *txemulator.TxTree, accounts map[tongo.AccountID]tlb.ShardAccount) (*core.Trace, error) {
	if !tree.TX.Msgs.InMsg.Exists {
		return nil, errors.New("there is no incoming message in emulation result")
	}
	m := tree.TX.Msgs.InMsg.Value.Value
	var a tlb.MsgAddress
	switch m.Info.SumType {
	case "IntMsgInfo":
		a = m.Info.IntMsgInfo.Dest
	case "ExtInMsgInfo":
		a = m.Info.ExtInMsgInfo.Dest
	default:
		return nil, errors.New("unknown message type in emulation result")
	}
	transaction, err := core.ConvertTransaction(int32(a.AddrStd.WorkchainId), tongo.Transaction{
		Transaction: tree.TX,
		BlockID:     tongo.BlockIDExt{BlockID: tongo.BlockID{Workchain: int32(a.AddrStd.WorkchainId)}},
	})
	if err != nil {
		return nil, err
	}
	t := &core.Trace{
		Transaction:       *transaction,
		AccountInterfaces: nil, //todo: do
		AdditionalInfo:    nil, //todo: do
	}
	for i := range tree.Children {
		child, err := emulatedTreeToTrace(tree.Children[i], accounts)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, child)
	}
	return t, nil
}
