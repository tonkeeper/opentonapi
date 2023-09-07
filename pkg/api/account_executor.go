package api

import (
	"context"
	"errors"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/tvm"
)

type shardsAccountExecutor struct {
	accounts     map[tongo.AccountID]tlb.ShardAccount
	configBase64 string
}

func (s shardsAccountExecutor) RunSmcMethodByID(ctx context.Context, accountID tongo.AccountID, methodID int, params tlb.VmStack) (uint32, tlb.VmStack, error) {
	w, ok := s.accounts[accountID]
	if !ok {
		return 0, nil, errors.New("address not found")
	}
	code, data := accountCode(w), accountData(w)
	if code == nil || data == nil {
		return 0, nil, errors.New("account not found")
	}
	codeBoc, err := code.ToBocBase64()
	if err != nil {
		return 0, nil, err
	}
	dataBoc, err := data.ToBocBase64()
	if err != nil {
		return 0, nil, err
	}
	e, err := tvm.NewEmulatorFromBOCsBase64(codeBoc, dataBoc, s.configBase64) //todo:libs
	if err != nil {
		return 0, nil, err
	}
	return e.RunSmcMethodByID(ctx, accountID, methodID, params)
}

func newSharedAccountExecutor(a map[tongo.AccountID]tlb.ShardAccount, configBase64 string) *shardsAccountExecutor {
	return &shardsAccountExecutor{accounts: a, configBase64: configBase64}
}

func accountCode(account tlb.ShardAccount) *boc.Cell {
	if account.Account.SumType == "AccountNone" {
		return nil
	}
	if account.Account.Account.Storage.State.SumType != "AccountActive" {
		return nil
	}
	code := account.Account.Account.Storage.State.AccountActive.StateInit.Code
	if !code.Exists {
		return nil
	}
	cell := code.Value.Value
	return &cell
}

func accountData(account tlb.ShardAccount) *boc.Cell {
	if account.Account.SumType == "AccountNone" {
		return nil
	}
	if account.Account.Account.Storage.State.SumType != "AccountActive" {
		return nil
	}
	data := account.Account.Account.Storage.State.AccountActive.StateInit.Data
	if !data.Exists {
		return nil
	}
	cell := data.Value.Value
	return &cell
}
