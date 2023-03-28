package api

import (
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertToRawAccount(account *core.Account) oas.RawAccount {
	rawAccount := oas.RawAccount{
		Address:           account.AccountAddress.ToRaw(),
		Balance:           account.TonBalance,
		LastTransactionLt: int64(account.LastTransactionLt),
		Status:            account.Status,
		Storage: oas.AccountStorageInfo{
			UsedCells:       account.Storage.UsedCells.Int64(),
			UsedBits:        account.Storage.UsedBits.Int64(),
			UsedPublicCells: account.Storage.UsedPublicCells.Int64(),
			LastPaid:        int64(account.Storage.LastPaid),
			DuePayment:      account.Storage.DuePayment,
		},
	}
	if account.ExtraBalances != nil {
		balances := make(map[string]string, len(account.ExtraBalances))
		for key, value := range account.ExtraBalances {
			balances[fmt.Sprintf("%v", key)] = fmt.Sprintf("%v", value)
		}
		rawAccount.ExtraBalance = oas.NewOptRawAccountExtraBalance(balances)
	}
	if account.Code != nil && len(account.Code) != 0 {
		rawAccount.Code = oas.NewOptString(fmt.Sprintf("%x", account.Code[:]))
	}
	if account.Data != nil {
		rawAccount.Data = oas.NewOptString(fmt.Sprintf("%x", account.Data[:]))
	}
	return rawAccount
}

func convertToAccount(info *core.AccountInfo) oas.Account {
	acc := oas.Account{
		Address:      info.Account.AccountAddress.ToRaw(),
		Balance:      info.Account.TonBalance,
		LastActivity: info.Account.LastActivityTime,
		Status:       info.Account.Status,
		Interfaces:   info.Account.Interfaces,
		GetMethods:   info.Account.GetMethods,
	}
	if info.Name != nil {
		acc.Name = oas.NewOptString(*info.Name)
	}
	if info.Icon != nil {
		acc.Icon = oas.NewOptString(*info.Icon)
	}
	if info.IsScam != nil {
		acc.IsScam = oas.NewOptBool(*info.IsScam)
	}
	if info.MemoRequired != nil {
		acc.MemoRequired = oas.NewOptBool(*info.MemoRequired)
	}
	return acc
}
