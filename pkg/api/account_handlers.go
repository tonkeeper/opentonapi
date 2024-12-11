package api

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/exp/slices"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/go-faster/jx"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/code"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/utils"
	walletTongo "github.com/tonkeeper/tongo/wallet"
	"golang.org/x/exp/maps"
)

func (h *Handler) GetBlockchainRawAccount(ctx context.Context, params oas.GetBlockchainRawAccountParams) (*oas.BlockchainRawAccount, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res, err := convertToRawAccount(rawAccount)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &res, nil
}

func (h *Handler) GetAccount(ctx context.Context, params oas.GetAccountParams) (*oas.Account, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.Account{
			Address: account.ID.ToRaw(),
			Status:  oas.AccountStatusNonexist,
		}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	ab, found := h.addressBook.GetAddressInfoByAddress(account.ID)
	var res oas.Account
	if found {
		res = convertToAccount(rawAccount, &ab, h.state)
	} else {
		res = convertToAccount(rawAccount, nil, h.state)
	}
	if rawAccount.ExtraBalances != nil {
		res.ExtraBalance = convertExtraCurrencies(rawAccount.ExtraBalances)
	}
	return &res, nil
}

func (h *Handler) GetAccounts(ctx context.Context, request oas.OptGetAccountsReq, params oas.GetAccountsParams) (*oas.Accounts, error) {
	if len(request.Value.AccountIds) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("empty list of ids"))
	}
	if !h.limits.isBulkQuantityAllowed(len(request.Value.AccountIds)) {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("the maximum number of accounts to request at once: %v", h.limits.BulkLimits))
	}
	var currencyPrice float64
	currency := strings.ToUpper(params.Currency.Value)
	if currency != "" {
		rates, err := h.ratesSource.GetRates(time.Now().Unix())
		if err == nil {
			currencyPrice = rates[currency]
		}
	}
	var ids []tongo.AccountID
	allAccountIDs := make(map[tongo.AccountID]struct{}, len(request.Value.AccountIds))
	for _, str := range request.Value.AccountIds {
		account, err := tongo.ParseAddress(str)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		ids = append(ids, account.ID)
		allAccountIDs[account.ID] = struct{}{}
	}
	accounts, err := h.storage.GetRawAccounts(ctx, ids)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	results := make(map[ton.AccountID]oas.Account, len(accounts))
	for _, account := range accounts {
		delete(allAccountIDs, account.AccountAddress)
		ab, found := h.addressBook.GetAddressInfoByAddress(account.AccountAddress)
		var res oas.Account
		if found {
			res = convertToAccount(account, &ab, h.state)
		} else {
			res = convertToAccount(account, nil, h.state)
		}
		if account.ExtraBalances != nil {
			res.ExtraBalance = convertExtraCurrencies(account.ExtraBalances)
		}
		results[account.AccountAddress] = res
	}
	// if we don't find an account, we return it with "nonexist" status
	for accountID := range allAccountIDs {
		account := oas.Account{
			Address:  accountID.ToRaw(),
			Status:   oas.AccountStatusNonexist,
			IsWallet: true,
		}
		results[accountID] = account
	}
	resp := &oas.Accounts{}
	for _, i := range ids {
		account := results[i]
		if currencyPrice != 0 {
			convertedAmount := float64(account.Balance/int64(ton.OneTON)) / currencyPrice
			currenciesBalance := map[string]jx.Raw{currency: jx.Raw(fmt.Sprintf("%f", convertedAmount))}
			account.CurrenciesBalance.SetTo(currenciesBalance)
		}
		resp.Accounts = append(resp.Accounts, account)
	}
	return resp, nil
}

func (h *Handler) GetBlockchainAccountTransactions(ctx context.Context, params oas.GetBlockchainAccountTransactionsParams) (*oas.Transactions, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	if params.BeforeLt.Value == 0 {
		params.BeforeLt.Value = 1 << 62
	}
	descendingOrder := true
	if params.SortOrder.Value == oas.GetBlockchainAccountTransactionsSortOrderAsc {
		descendingOrder = false
	}
	txs, err := h.storage.GetAccountTransactions(ctx, account.ID, int(params.Limit.Value),
		uint64(params.BeforeLt.Value), uint64(params.AfterLt.Value), descendingOrder)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.Transactions{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := oas.Transactions{
		Transactions: make([]oas.Transaction, len(txs)),
	}
	accountObject, err := h.storage.GetRawAccount(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	for i, tx := range txs {
		result.Transactions[i] = convertTransaction(*tx, accountObject.Interfaces, h.addressBook)
	}
	return &result, nil
}

func getMethodCacheKey(accountID ton.AccountID, methodName string, lt uint64, args []string) (string, error) {
	d := xxhash.New()
	var x [8]byte
	binary.LittleEndian.PutUint64(x[:], lt)
	if _, err := d.Write(x[:]); err != nil {
		return "", err
	}
	if _, err := d.WriteString(methodName); err != nil {
		return "", err
	}
	for _, arg := range args {
		if _, err := d.WriteString(arg + "|"); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%s-%d", accountID.ToRaw(), d.Sum64()), nil
}

func (h *Handler) ExecGetMethodForBlockchainAccount(ctx context.Context, params oas.ExecGetMethodForBlockchainAccountParams) (*oas.MethodExecutionResult, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	contract, err := h.storage.GetContract(ctx, account.ID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}
	// TODO: remove parameter after user migration
	if params.FixOrder.IsSet() && params.FixOrder.Value == true && len(params.Args) > 1 {
		slices.Reverse(params.Args)
	}
	key, err := getMethodCacheKey(account.ID, params.MethodName, contract.LastTransactionLt, params.Args)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if result, ok := h.getMethodsCache.Get(key); ok {
		return result, nil
	}
	stack := make([]tlb.VmStackValue, 0, len(params.Args))
	for i := len(params.Args) - 1; i >= 0; i-- {
		r, err := stringToTVMStackRecord(params.Args[i])
		if err != nil {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("can't parse arg '%v' as any TVMStackValue", params.Args[i]))
		}
		stack = append(stack, r) // we need to put the arguments on the stack in reverse order
	}
	// RunSmcMethodByID fetches the contract from the storage on its own,
	// and it can happen that the contract has been changed and has another lt,
	// in this case, we get a correct result from the executor,
	// but we update a previous cache entry that won't be used anymore.
	exitCode, stack, err := h.executor.RunSmcMethodByID(ctx, account.ID, utils.MethodIdFromName(params.MethodName), stack)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := oas.MethodExecutionResult{
		Success:  exitCode == 0 || exitCode == 1,
		ExitCode: int(exitCode),
		Stack:    make([]oas.TvmStackRecord, 0, len(stack)),
	}
	for i := range stack {
		value, err := convertTvmStackValue(stack[i])
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		result.Stack = append(result.Stack, value)
	}
	for _, decoder := range abi.KnownGetMethodsDecoder[params.MethodName] {
		_, v, err := decoder(stack)
		if err == nil {
			value, err := json.Marshal(v)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			result.SetDecoded(g.ChangeJsonKeys(value, g.CamelToSnake))
			break
		}
	}
	h.getMethodsCache.Set(key, &result)
	return &result, nil
}

func (h *Handler) SearchAccounts(ctx context.Context, params oas.SearchAccountsParams) (*oas.FoundAccounts, error) {
	attachedAccounts := h.addressBook.SearchAttachedAccountsByPrefix(params.Name)
	parsedAccounts := make(map[tongo.AccountID]addressbook.AttachedAccount)
	for _, account := range attachedAccounts {
		if account.Symbol != "" {
			trust := h.spamFilter.JettonTrust(account.Wallet, account.Symbol, account.Name, account.Preview)
			if trust == core.TrustBlacklist {
				continue
			}
		}
		parsedAccounts[account.Wallet] = account
	}
	accounts := maps.Values(parsedAccounts)
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Weight == accounts[j].Weight {
			return len(accounts[i].Name) < len(accounts[j].Name)
		}
		return accounts[i].Weight > accounts[j].Weight
	})
	converted := make([]oas.FoundAccountsAddressesItem, len(accounts))
	for idx, account := range accounts {
		converted[idx] = oas.FoundAccountsAddressesItem{
			Address: account.Wallet.ToRaw(),
			Name:    account.Name,
			Preview: account.Preview,
		}
	}
	return &oas.FoundAccounts{Addresses: converted}, nil
}

// ReindexAccount updates internal cache for a particular account.
func (h *Handler) ReindexAccount(ctx context.Context, params oas.ReindexAccountParams) error {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return toError(http.StatusBadRequest, err)
	}
	if err = h.storage.ReindexAccount(ctx, account.ID); err != nil {
		return toError(http.StatusInternalServerError, err)
	}
	return nil
}

func (h *Handler) GetAccountDnsExpiring(ctx context.Context, params oas.GetAccountDnsExpiringParams) (*oas.DnsExpiring, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var period *int
	if params.Period.Set {
		period = &params.Period.Value
	}
	dnsExpiring, err := h.storage.GetDnsExpiring(ctx, account.ID, period)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.DnsExpiring{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
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
		return nil, toError(http.StatusInternalServerError, err)
	}
	for _, dns := range dnsExpiring {
		dei := oas.DnsExpiringItemsItem{
			Name:       dns.Name,
			ExpiringAt: dns.ExpiringAt,
		}
		if dns.DnsItem != nil {
			for _, n := range nfts {
				if n.Address == dns.DnsItem.Address {
					dei.DNSItem = oas.NewOptNftItem(h.convertNFT(ctx, n, h.addressBook, h.metaCache))
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

func (h *Handler) GetAccountPublicKey(ctx context.Context, params oas.GetAccountPublicKeyParams) (*oas.GetAccountPublicKeyOK, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	pubKey, err := h.storage.GetWalletPubKey(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		state, err := h.storage.GetRawAccount(ctx, account.ID)
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		pubKey, err = pubkeyFromCodeData(state.Code, state.Data)
		if errors.Is(err, unknownWalletVersionError) {
			return nil, toError(http.StatusBadRequest, err)
		}
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
	}
	return &oas.GetAccountPublicKeyOK{PublicKey: hex.EncodeToString(pubKey)}, nil
}

func (h *Handler) GetAccountSubscriptions(ctx context.Context, params oas.GetAccountSubscriptionsParams) (*oas.Subscriptions, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	subscriptions, err := h.storage.GetSubscriptions(ctx, account.ID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
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

func (h *Handler) GetAccountTraces(ctx context.Context, params oas.GetAccountTracesParams) (*oas.TraceIDs, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.SearchTraces(ctx, account.ID, params.Limit.Value, optIntToPointer(params.BeforeLt), nil, nil, false)
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusInternalServerError, err)
	}
	traces := oas.TraceIDs{
		Traces: make([]oas.TraceID, 0, len(traceIDs)),
	}
	for _, traceID := range traceIDs {
		traces.Traces = append(traces.Traces, oas.TraceID{
			ID:    traceID.Hash.Hex(),
			Utime: int64(traceID.Lt),
		})
	}
	return &traces, nil
}

func (h *Handler) GetAccountDiff(ctx context.Context, params oas.GetAccountDiffParams) (*oas.GetAccountDiffOK, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	balanceChange, err := h.storage.GetAccountDiff(ctx, account.ID, params.StartDate, params.EndDate)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.GetAccountDiffOK{BalanceChange: balanceChange}, nil
}

func (h *Handler) GetAccountNftHistory(ctx context.Context, params oas.GetAccountNftHistoryParams) (*oas.AccountEvents, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.GetAccountNftsHistory(ctx, account.ID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	events, lastLT, err := h.convertNftHistory(ctx, account.ID, traceIDs, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}

func (h *Handler) BlockchainAccountInspect(ctx context.Context, params oas.BlockchainAccountInspectParams) (*oas.BlockchainAccountInspect, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, account.ID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	cells, err := boc.DeserializeBoc(rawAccount.Code)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if len(cells) != 1 {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("invalid boc with code"))
	}
	codeHash, err := cells[0].Hash()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	methods, err := code.ParseContractMethods(rawAccount.Code)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	resp := oas.BlockchainAccountInspect{
		Code:     hex.EncodeToString(rawAccount.Code),
		CodeHash: hex.EncodeToString(codeHash),
		Compiler: oas.NewOptBlockchainAccountInspectCompiler(oas.BlockchainAccountInspectCompilerFunc),
	}
	for _, methodID := range methods {
		if method, ok := code.Methods[methodID]; ok {
			resp.Methods = append(resp.Methods, oas.BlockchainAccountInspectMethodsItem{
				ID:     methodID,
				Method: string(method),
			})
		}
	}
	return &resp, nil
}

func (h *Handler) GetAccountMultisigs(ctx context.Context, params oas.GetAccountMultisigsParams) (*oas.Multisigs, error) {
	accountID, err := ton.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	multisigs, err := h.storage.GetAccountMultisigs(ctx, accountID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var converted []oas.Multisig
	for _, multisig := range multisigs {
		oasMultisig, err := h.convertMultisig(ctx, multisig)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		converted = append(converted, *oasMultisig)
	}
	return &oas.Multisigs{Multisigs: converted}, nil
}

var unknownWalletVersionError = fmt.Errorf("unknown wallet version")

func pubkeyFromCodeData(code, data []byte) ([]byte, error) {
	if len(code) == 0 {
		return nil, unknownWalletVersionError
	}
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
		return nil, unknownWalletVersionError
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
		return nil, unknownWalletVersionError
	}
}

func (h *Handler) AddressParse(ctx context.Context, params oas.AddressParseParams) (*oas.AddressParseOK, error) {
	address, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var res oas.AddressParseOK
	res.RawForm = address.ID.ToRaw()
	res.Bounceable.B64url = address.ID.ToHuman(true, false)
	res.NonBounceable.B64url = address.ID.ToHuman(false, false)
	b, _ := base64.URLEncoding.DecodeString(res.Bounceable.B64url)
	res.Bounceable.B64 = base64.StdEncoding.EncodeToString(b)
	b, _ = base64.URLEncoding.DecodeString(res.NonBounceable.B64url)
	res.NonBounceable.B64 = base64.StdEncoding.EncodeToString(b)
	if strings.Contains(params.AccountID, ":") {
		res.GivenType = "raw_form"
	} else if strings.Contains(params.AccountID, ".") {
		res.GivenType = "dns"
	} else if address.Bounce {
		res.GivenType = "friendly_bounceable"
	} else {
		res.GivenType = "friendly_non_bounceable"
	}
	return &res, nil //todo: add testnet_only
}
