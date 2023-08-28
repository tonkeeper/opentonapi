package api

import (
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertToRawAccount(account *core.Account) oas.BlockchainRawAccount {
	rawAccount := oas.BlockchainRawAccount{
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
		rawAccount.ExtraBalance = oas.NewOptBlockchainRawAccountExtraBalance(balances)
	}
	if account.Code != nil && len(account.Code) != 0 {
		rawAccount.Code = oas.NewOptString(fmt.Sprintf("%x", account.Code[:]))
	}
	if account.Data != nil {
		rawAccount.Data = oas.NewOptString(fmt.Sprintf("%x", account.Data[:]))
	}
	return rawAccount
}

func convertToAccount(account *core.Account, ab *addressbook.KnownAddress, state chainState) oas.Account {
	acc := oas.Account{
		Address:      account.AccountAddress.ToRaw(),
		Balance:      account.TonBalance,
		LastActivity: account.LastActivityTime,
		Status:       account.Status,
		Interfaces:   account.Interfaces,
		GetMethods:   account.GetMethods,
	}
	if state.CheckIsSuspended(account.AccountAddress) {
		acc.IsSuspended.SetTo(true)
	}
	if ab == nil {
		return acc
	}
	acc.IsScam = oas.NewOptBool(ab.IsScam)
	if len(ab.Name) > 0 {
		acc.Name = oas.NewOptString(ab.Name)
	}
	if len(ab.Image) > 0 {
		acc.Icon = oas.NewOptString(ab.Image)
	}
	acc.MemoRequired = oas.NewOptBool(ab.RequireMemo)
	return acc
}
