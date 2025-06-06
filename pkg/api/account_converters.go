package api

import (
	"fmt"
	"math/big"
	"sort"

	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/opentonapi/pkg/references"

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
		rawAccount.ExtraBalance = convertExtraCurrencies(account.ExtraBalances)
	}
	if account.Code != nil && len(account.Code) != 0 {
		rawAccount.Code = oas.NewOptString(fmt.Sprintf("%x", account.Code[:]))
	}
	if account.Data != nil {
		rawAccount.Data = oas.NewOptString(fmt.Sprintf("%x", account.Data[:]))
	}
	return rawAccount, nil
}

func convertExtraCurrencies(extraBalances core.ExtraCurrencies) []oas.ExtraCurrency {
	res := make([]oas.ExtraCurrency, 0, len(extraBalances))
	for k, v := range extraBalances {
		amount := big.Int(v)
		meta := references.GetExtraCurrencyMeta(k)
		cur := oas.ExtraCurrency{
			Amount: amount.String(),
			Preview: oas.EcPreview{
				ID:       k,
				Symbol:   meta.Symbol,
				Decimals: meta.Decimals,
				Image:    meta.Image,
			},
		}
		res = append(res, cur)
	}
	return res
}

func convertToAccount(account *core.Account, ab *addressbook.KnownAddress, state chainState, spamFilter SpamFilter) oas.Account {
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
	} else {
		for _, i := range account.Interfaces {
			if i.Implements(abi.Wallet) {
				acc.IsWallet = true
				break
			}
		}
	}
	trust := spamFilter.AccountTrust(account.AccountAddress)
	if trust == core.TrustBlacklist {
		acc.IsScam = oas.NewOptBool(true)
	}
	if ab == nil {
		return acc
	}
	if !acc.IsScam.Value {
		acc.IsScam = oas.NewOptBool(ab.IsScam)
	}
	if len(ab.Name) > 0 {
		acc.Name = oas.NewOptString(ab.Name)
	}
	if len(ab.Image) > 0 {
		acc.Icon = oas.NewOptString(imgGenerator.DefaultGenerator.GenerateImageUrl(ab.Image, 200, 200))
	}
	acc.MemoRequired = oas.NewOptBool(ab.RequireMemo)
	return acc
}

func convertToWallet(account *core.Account, ab *addressbook.KnownAddress, state chainState, stats core.AccountStat, plugins []core.Plugin) oas.Wallet {
	wallet := oas.Wallet{
		Address:      account.AccountAddress.ToRaw(),
		Balance:      account.TonBalance,
		LastActivity: account.LastActivityTime,
		GetMethods:   []string{},
		Status:       oas.AccountStatus(account.Status),
		Stats: oas.WalletStats{
			NftsCount:     stats.NftsCount,
			JettonsCount:  stats.JettonsCount,
			MultisigCount: stats.MultisigCount,
			StakingCount:  stats.StakingCount,
		},
	}
	for _, plugin := range plugins {
		wallet.Plugins = append(wallet.Plugins, oas.WalletPlugin{
			Address: plugin.AccountID.ToRaw(),
			Type:    plugin.Type,
			Status:  oas.AccountStatus(plugin.Status),
		})
	}
	if state.CheckIsSuspended(account.AccountAddress) {
		wallet.IsSuspended.SetTo(true)
	}
	if ab == nil {
		return wallet
	}
	if len(ab.Name) > 0 {
		wallet.Name = oas.NewOptString(ab.Name)
	}
	if len(ab.Image) > 0 {
		wallet.Icon = oas.NewOptString(imgGenerator.DefaultGenerator.GenerateImageUrl(ab.Image, 200, 200))
	}
	return wallet
}
