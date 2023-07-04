package api

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	jsoniter "github.com/json-iterator/go"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	walletTongo "github.com/tonkeeper/tongo/wallet"
)

func (h Handler) GetAccount(ctx context.Context, params oas.GetAccountParams) (oas.GetAccountRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	account, err := h.storage.GetRawAccount(ctx, accountID)
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
	var res oas.Account
	if found {
		res = convertToAccount(account, &ab)
	} else {
		res = convertToAccount(account, nil)
	}
	return &res, nil
}

func (h Handler) GetAccounts(ctx context.Context, req oas.OptGetAccountsReq) (r oas.GetAccountsRes, _ error) {
	if len(req.Value.AccountIds) == 0 {
		return &oas.BadRequest{Error: "empty list of ids"}, nil
	}
	if !h.limits.isBulkQuantityAllowed(len(req.Value.AccountIds)) {
		return &oas.BadRequest{Error: fmt.Sprintf("the maximum number of accounts to request at once: %v", h.limits.BulkLimits)}, nil
	}
	var ids []tongo.AccountID
	allAccountIDs := make(map[tongo.AccountID]struct{}, len(req.Value.AccountIds))
	for _, str := range req.Value.AccountIds {
		accountID, err := tongo.ParseAccountID(str)
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
		ids = append(ids, accountID)
		allAccountIDs[accountID] = struct{}{}
	}
	accounts, err := h.storage.GetRawAccounts(ctx, ids)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	results := make([]oas.Account, 0, len(accounts))
	for _, account := range accounts {
		delete(allAccountIDs, account.AccountAddress)
		ab, found := h.addressBook.GetAddressInfoByAddress(account.AccountAddress)
		var res oas.Account
		if found {
			res = convertToAccount(account, &ab)
		} else {
			res = convertToAccount(account, nil)
		}
		results = append(results, res)
	}
	// if we don't find an account, we return it with "nonexist" status
	for accountID := range allAccountIDs {
		account := oas.Account{
			Address: accountID.ToRaw(),
			Status:  string(tlb.AccountNone),
		}
		results = append(results, account)
	}
	return &oas.Accounts{Accounts: results}, nil
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
			value, err := jsoniter.Marshal(v)
			if err != nil {
				return nil, err
			}
			result.SetDecoded(value)
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
		result.Transactions[i] = convertTransaction(*tx, h.addressBook)
	}
	return &result, nil
}

func (h Handler) GetSearchAccounts(ctx context.Context, params oas.GetSearchAccountsParams) (res oas.GetSearchAccountsRes, err error) {
	accounts := h.addressBook.SearchAttachedAccountsByPrefix(params.Name)
	var response oas.FoundAccounts
	for _, account := range accounts {
		response.Addresses = append(response.Addresses, oas.FoundAccountsAddressesItem{
			Address: account.Wallet,
			Name:    account.Name,
		})
	}

	return &response, nil
}

// ReindexAccount updates internal cache for a particular account.
func (h Handler) ReindexAccount(ctx context.Context, params oas.ReindexAccountParams) (r oas.ReindexAccountRes, err error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if err = h.storage.ReindexAccount(ctx, accountID); err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &oas.ReindexAccountOK{}, nil
}

func (h Handler) GetDnsExpiring(ctx context.Context, params oas.GetDnsExpiringParams) (r oas.GetDnsExpiringRes, _ error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	dnsExpiring, err := h.storage.GetDnsExpiring(ctx, accountID, params.Period.Value)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	accounts := make([]tongo.AccountID, 0, len(dnsExpiring))
	var response oas.DnsExpiring
	if len(dnsExpiring) == 0 {
		return &response, nil
	}
	for _, dns := range dnsExpiring {
		if dns.DnsItem != nil {
			accounts = append(accounts, dns.DnsItem.Address)
		}
	}
	nfts, err := h.storage.GetNFTs(ctx, accounts)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	for _, dns := range dnsExpiring {
		dei := oas.DnsExpiringItemsItem{
			Name:       dns.Name,
			ExpiringAt: dns.ExpiringAt,
		}
		if dns.DnsItem != nil {
			for _, n := range nfts {
				if n.Address == dns.DnsItem.Address {
					dei.DNSItem = oas.NewOptNftItem(convertNFT(ctx, n, h.addressBook, h.previewGenerator, h.metaCache))
					break
				}
			}
		}
		response.Items = append(response.Items, dei)
	}
	sort.Slice(response.Items, func(i, j int) bool {
		return response.Items[i].ExpiringAt > response.Items[j].ExpiringAt
	})

	return &response, nil
}

func (h Handler) GetPublicKeyByAccountID(ctx context.Context, params oas.GetPublicKeyByAccountIDParams) (r oas.GetPublicKeyByAccountIDRes, _ error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	pubKey, err := h.storage.GetWalletPubKey(ctx, accountID)
	if err != nil {
		state, err := h.storage.GetRawAccount(ctx, accountID)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		pubKey, err = pubkeyFromCodeData(state.Code, state.Data)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
	}
	return &oas.GetPublicKeyByAccountIDOK{PublicKey: hex.EncodeToString(pubKey)}, nil
}

func (h Handler) GetSubscriptionsByAccount(ctx context.Context, params oas.GetSubscriptionsByAccountParams) (res oas.GetSubscriptionsByAccountRes, err error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	subscriptions, err := h.storage.GetSubscriptions(ctx, accountID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var response oas.Subscriptions
	for _, subscription := range subscriptions {
		response.Subscriptions = append(response.Subscriptions, oas.Subscription{
			Address:            subscription.AccountID.ToRaw(),
			WalletAddress:      subscription.WalletAccountID.ToRaw(),
			BeneficiaryAddress: subscription.BeneficiaryAccountID.ToRaw(),
			Amount:             subscription.Amount,
			Period:             subscription.Period,
			StartTime:          subscription.StartTime,
			Timeout:            subscription.Timeout,
			LastPaymentTime:    subscription.LastPaymentTime,
			LastRequestTime:    subscription.LastRequestTime,
			SubscriptionID:     subscription.SubscriptionID,
			FailedAttempts:     subscription.FailedAttempts,
		})
	}
	return &response, nil
}

func pubkeyFromCodeData(code, data []byte) ([]byte, error) {
	cells, err := boc.DeserializeBoc(code)
	if err != nil {
		return nil, err
	}
	if len(cells) != 1 {
		return nil, fmt.Errorf("invalid boc with code")
	}
	codeHash, err := cells[0].Hash()
	if err != nil {
		return nil, err
	}
	ver, ok := walletTongo.GetVerByCodeHash([32]byte(codeHash))
	if !ok {
		return nil, fmt.Errorf("unknown wallet version")
	}
	switch ver {
	case walletTongo.V3R1:
		var dataBody walletTongo.DataV3
		cells, err = boc.DeserializeBoc(data)
		if err != nil {
			return nil, err
		}
		err = tlb.Unmarshal(cells[0], &dataBody)
		if err != nil {
			return nil, err
		}
		return dataBody.PublicKey[:], nil
	default:
		return nil, fmt.Errorf("unknown wallet version")
	}

}
