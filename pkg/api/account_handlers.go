package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

func (h Handler) GetAccount(ctx context.Context, params oas.GetAccountParams) (oas.GetAccountRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	info, err := h.storage.GetAccountInfo(ctx, accountID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.Account{
			Address: accountID.ToRaw(),
			Status:  string(tlb.AccountNone),
		}, nil
	}
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	ab, found := h.addressBook.GetAddressInfoByAddress(accountID)
	if found {
		info.IsScam = &ab.IsScam
		if len(ab.Name) > 0 {
			info.Name = &ab.Name
		}
		if len(ab.Image) > 0 {
			info.Icon = &ab.Image
		}
		info.MemoRequired = &ab.RequireMemo
	}
	res := convertToAccount(info)
	return &res, nil
}

func (h Handler) GetRawAccount(ctx context.Context, params oas.GetRawAccountParams) (r oas.GetRawAccountRes, _ error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, accountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	res := convertToRawAccount(rawAccount)
	return &res, nil
}

func (h Handler) ExecGetMethod(ctx context.Context, params oas.ExecGetMethodParams) (oas.ExecGetMethodRes, error) {
	id, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	var stack tlb.VmStack
	for _, p := range params.Args {
		r, err := stringToTVMStackRecord(p)
		if err != nil {
			return &oas.BadRequest{Error: fmt.Sprintf("can't parse arg '%v' as any TVMStackValue", p)}, nil
		}
		stack = append(stack, r)
	}
	exitCode, stack, err := h.executor.RunSmcMethod(ctx, id, params.MethodName, stack)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	result := oas.MethodExecutionResult{
		Success:  exitCode == 0 || exitCode == 1,
		ExitCode: int(exitCode),
		Stack:    make([]oas.TvmStackRecord, 0, len(stack)),
	}
	for i := range stack {
		value, err := convertTvmStackValue(stack[i])
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		result.Stack = append(result.Stack, value)
	}
	for _, decoder := range abi.KnownGetMethodsDecoder[params.MethodName] {
		_, v, err := decoder(stack)
		if err == nil {
			result.SetDecoded(oas.NewOptMethodExecutionResultDecoded(anyToJSONRawMap(v)))
			break
		}
	}

	return &result, nil
}

func (h Handler) GetAccountTransactions(ctx context.Context, params oas.GetAccountTransactionsParams) (r oas.GetAccountTransactionsRes, err error) {
	a, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if params.BeforeLt.Value == 0 {
		params.BeforeLt.Value = 1 << 62
	}
	txs, err := h.storage.GetAccountTransactions(ctx, a, int(params.Limit.Value), uint64(params.BeforeLt.Value), uint64(params.AfterLt.Value))
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	result := oas.Transactions{
		Transactions: make([]oas.Transaction, len(txs)),
	}
	for i, tx := range txs {
		result.Transactions[i] = convertTransaction(*tx)
	}
	return &result, nil
}
