package api

import (
	"fmt"
	"sort"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertToRawAccount(account *core.Account) (oas.BlockchainRawAccount, error) {
	rawAccount := oas.BlockchainRawAccount{
		Address:           account.AccountAddress.ToRaw(),
		Balance:           account.TonBalance,
		LastTransactionLt: int64(account.LastTransactionLt),
		Status:            oas.AccountStatus(account.Status),
		Storage: oas.AccountStorageInfo{
			UsedCells:       account.Storage.UsedCells.Int64(),
			UsedBits:        account.Storage.UsedBits.Int64(),
			UsedPublicCells: account.Storage.UsedPublicCells.Int64(),
			LastPaid:        int64(account.Storage.LastPaid),
			DuePayment:      account.Storage.DuePayment,
		},
	}
	if account.LastTransactionHash != [32]byte{} {
		rawAccount.LastTransactionHash = oas.NewOptString(account.LastTransactionHash.Hex())
	}
	if account.FrozenHash != nil {
		rawAccount.FrozenHash = oas.NewOptString(account.FrozenHash.Hex())
	}
	if len(account.Libraries) > 0 {
		rawAccount.Libraries = make([]oas.BlockchainRawAccountLibrariesItem, 0, len(account.Libraries))
		keys := make([]string, 0, len(account.Libraries))
		values := make(map[string]*core.SimpleLib, len(account.Libraries))
		for key, value := range account.Libraries {
			hex := key.Hex()
			keys = append(keys, hex)
			values[hex] = value
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := values[key]
			bocHex, err := value.Root.ToBocString()
			if err != nil {
				// this should never happen, but anyway
				return oas.BlockchainRawAccount{}, err
			}
			rawAccount.Libraries = append(rawAccount.Libraries, oas.BlockchainRawAccountLibrariesItem{
				Public: value.Public,
				Root:   bocHex,
			})
		}
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
	return rawAccount, nil
}

func convertToAccount(account *core.Account, ab *addressbook.KnownAddress, state chainState) oas.Account {
	acc := oas.Account{
		Address:      account.AccountAddress.ToRaw(),
		Balance:      account.TonBalance,
		LastActivity: account.LastActivityTime,
		Status:       oas.AccountStatus(account.Status),
		Interfaces:   make([]string, len(account.Interfaces)),
		GetMethods:   account.GetMethods,
	}
	for i, iface := range account.Interfaces {
		acc.Interfaces[i] = iface.String()
	}
	if state.CheckIsSuspended(account.AccountAddress) {
		acc.IsSuspended.SetTo(true)
	}
	if account.Status == tlb.AccountUninit || account.Status == tlb.AccountNone {
		acc.IsWallet = true
	}
	for _, i := range account.Interfaces {
		if i.Implements(abi.Wallet) {
			acc.IsWallet = true
			break
		}
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
