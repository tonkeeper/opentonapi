package api

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"github.com/cespare/xxhash/v2"
	"github.com/go-faster/jx"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/utils"
	walletTongo "github.com/tonkeeper/tongo/wallet"
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
			Address:  account.ID.ToRaw(),
			Status:   oas.AccountStatusNonexist,
			IsWallet: true,
		}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	ab, found := h.addressBook.GetAddressInfoByAddress(account.ID)
	var res oas.Account
	if found {
		res = convertToAccount(rawAccount, &ab, h.state, h.spamFilter)
	} else {
		res = convertToAccount(rawAccount, nil, h.state, h.spamFilter)
	}
	if strings.HasSuffix(params.AccountID, ".ton") {
		trust := h.spamFilter.TonDomainTrust(params.AccountID)
		if trust == core.TrustBlacklist {
			res.IsScam = oas.NewOptBool(true)
		}
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
			res = convertToAccount(account, &ab, h.state, h.spamFilter)
		} else {
			res = convertToAccount(account, nil, h.state, h.spamFilter)
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
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
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
		stack = append(stack, r) // arguments go in reverse order
	}
	result, err := h.execGetMethod(ctx, account.ID, params.MethodName, stack)
	if err != nil {
		return nil, err
	}
	h.getMethodsCache.Set(key, result)
	return result, nil
}

func (h *Handler) ExecGetMethodWithBodyForBlockchainAccount(ctx context.Context, request oas.OptExecGetMethodWithBodyForBlockchainAccountReq, params oas.ExecGetMethodWithBodyForBlockchainAccountParams) (*oas.MethodExecutionResult, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	contract, err := h.storage.GetContract(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	args := request.Value.Args
	convertedArgs := make([]string, len(args))
	for idx, item := range args {
		convertedArgs[idx] = item.Value
	}
	key, err := getMethodCacheKey(account.ID, params.MethodName, contract.LastTransactionLt, convertedArgs)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if result, ok := h.getMethodsCache.Get(key); ok {
		return result, nil
	}
	stack := make([]tlb.VmStackValue, 0, len(args))
	for i := len(args) - 1; i >= 0; i-- {
		r, err := parseExecGetMethodArgs(args[i])
		if err != nil {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("can't parse arg '%v': %v", args[i], err))
		}
		stack = append(stack, r)
	}
	result, err := h.execGetMethod(ctx, account.ID, params.MethodName, stack)
	if err != nil {
		return nil, err
	}
	h.getMethodsCache.Set(key, result)
	return result, nil
}

func (h *Handler) SearchAccounts(ctx context.Context, params oas.SearchAccountsParams) (*oas.FoundAccounts, error) {
	attachedAccounts := h.addressBook.SearchAttachedAccountsByPrefix(params.Name)
	accounts := make([]addressbook.AttachedAccount, 0, len(attachedAccounts))
	for _, account := range attachedAccounts {
		if account.Symbol != "" {
			trust := h.spamFilter.JettonTrust(account.Wallet, account.Symbol, account.Name, account.Preview)
			if trust == core.TrustBlacklist {
				continue
			}
		}
		trust := h.spamFilter.AccountTrust(account.Wallet)
		if trust == core.TrustBlacklist {
			continue
		}
		accounts = append(accounts, account)
	}
	converted := make([]oas.FoundAccountsAddressesItem, len(accounts))
	for idx, account := range accounts {
		converted[idx] = oas.FoundAccountsAddressesItem{
			Address: account.Wallet.ToRaw(),
			Name:    account.Name,
			Preview: account.Preview,
			Trust:   oas.TrustType(account.Trust),
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
	var wg sync.WaitGroup
	wg.Add(1)
	var nftsScamData map[ton.AccountID]core.TrustType
	go func() {
		defer wg.Done()
		nftsScamData, err = h.spamFilter.GetNftsScamData(ctx, accounts)
		if err != nil {
			h.logger.Warn("error getting nft scam data", zap.Error(err))
		}
	}()
	nfts, err := h.storage.GetNFTs(ctx, accounts)
	wg.Wait()
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
					dei.DNSItem = oas.NewOptNftItem(h.convertNFT(ctx, n, h.addressBook, h.metaCache, nftsScamData[n.Address]))
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
	subscriptionsV1, err := h.storage.GetSubscriptionsV1(ctx, account.ID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	subscriptionsV2, err := h.storage.GetSubscriptionsV2(ctx, account.ID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var response oas.Subscriptions
	for _, subscription := range subscriptionsV1 {
		sub := h.convertSubscriptionsV1(ctx, subscription)
		response.Subscriptions = append(response.Subscriptions, sub)
	}
	for _, subscription := range subscriptionsV2 {
		sub := h.convertSubscriptionsV2(ctx, subscription)
		response.Subscriptions = append(response.Subscriptions, sub)
	}
	return &response, nil
}

func (h *Handler) GetAccountTraces(ctx context.Context, params oas.GetAccountTracesParams) (*oas.TraceIDs, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.SearchTraces(ctx, account.ID, params.Limit.Value, optIntToPointer(params.BeforeLt), nil, nil, nil, false, true)
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

func (h *Handler) GetAccountNftHistory(ctx context.Context, params oas.GetAccountNftHistoryParams) (*oas.NftOperations, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	history, err := h.storage.GetAccountNftsHistory(ctx, account.ID, params.Limit, optIntToPointer(params.BeforeLt), nil, nil)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NftOperations{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := oas.NftOperations{}
	for _, op := range history {
		res.Operations = append(res.Operations, h.convertNftOperation(ctx, op))
		if len(res.Operations) == params.Limit {
			res.NextFrom = oas.NewOptInt64(int64(op.Lt))
		}
	}
	return &res, nil
}

func (h *Handler) BlockchainAccountInspect(ctx context.Context, params oas.BlockchainAccountInspectParams) (*oas.BlockchainAccountInspect, error) {
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
	info, err := abi.GetCodeInfo(ctx, rawAccount.Code, h.storage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	source, _ := h.verifierSource.GetAccountSource(account.ID)
	sourceFiles := make([]oas.SourceFile, len(source.Files))
	for idx, file := range source.Files {
		sourceFiles[idx] = oas.SourceFile{
			Name:             file.Name,
			Content:          file.Content,
			IsEntrypoint:     file.IsEntrypoint,
			IsStdLib:         file.IsStdLib,
			IncludeInCommand: file.IncludeInCommand,
		}
	}
	compiler := oas.BlockchainAccountInspectCompilerFunc
	if source.Compiler != "" {
		compiler = oas.BlockchainAccountInspectCompiler(source.Compiler)
	}
	resp := oas.BlockchainAccountInspect{
		Code:     hex.EncodeToString(rawAccount.Code),
		CodeHash: hex.EncodeToString(codeHash),
		Compiler: compiler,
	}
	if source.DisassembledCode != "" {
		resp.DisassembledCode = oas.NewOptString(source.DisassembledCode)
	}
	if len(sourceFiles) > 0 {
		resp.Source = oas.NewOptSource(oas.Source{Files: sourceFiles})
	}
	knownMethods := make(map[int64]string)
	for _, name := range maps.Keys(abi.KnownGetMethodsDecoder) {
		knownMethods[int64(utils.MethodIdFromName(name))] = name
	}
	for _, methodID := range maps.Keys(info.Methods) {
		if method, ok := knownMethods[methodID]; ok {
			resp.Methods = append(resp.Methods, oas.Method{
				ID:     methodID,
				Method: method,
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

func (h *Handler) GetAccountExtraCurrencyHistoryByID(ctx context.Context, params oas.GetAccountExtraCurrencyHistoryByIDParams) (*oas.AccountEvents, error) {
	return &oas.AccountEvents{}, nil
}

func (h *Handler) execGetMethod(ctx context.Context, accountID ton.AccountID, methodName string, args []tlb.VmStackValue) (*oas.MethodExecutionResult, error) {
	// Execute the smart contract method by account ID and method name.
	// Note: RunSmcMethodByID fetches the latest state of the contract.
	// If the contract has changed and has a different logical time (lt),
	// we may return a valid result, but cache an outdated entry.
	exitCode, stack, err := h.executor.RunSmcMethodByID(ctx, accountID, utils.MethodIdFromName(methodName), args)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
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
	for _, decoder := range abi.KnownGetMethodsDecoder[methodName] {
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
	return &result, nil
}

func (h *Handler) convertSubscriptionsV2(ctx context.Context, sub core.SubscriptionV2) oas.Subscription {
	res := oas.Subscription{
		Type:           "v2",
		Period:         sub.Period,
		SubscriptionID: fmt.Sprintf("%d", sub.SubscriptionID),
		Wallet:         convertAccountAddress(sub.WalletAccountID, h.addressBook),
		NextChargeAt:   sub.ChargeDate,
	}
	res.Address.SetTo(sub.AccountID.ToRaw())
	res.Beneficiary.SetTo(convertAccountAddress(sub.WithdrawAccountID, h.addressBook))
	res.Admin.SetTo(convertAccountAddress(sub.AdminAccountID, h.addressBook))
	switch sub.ContractState {
	case 0:
		res.Status = oas.SubscriptionStatusNotReady
	case 1:
		if time.Now().Unix() > sub.ChargeDate+sub.GracePeriod { // grace period expired
			res.Status = oas.SubscriptionStatusCancelled
		} else {
			res.Status = oas.SubscriptionStatusActive
		}
	case 2:
		res.Status = oas.SubscriptionStatusCancelled
	}
	if len(sub.Metadata) > 0 {
		res.Metadata.SetEncryptedBinary(fmt.Sprintf("%x", sub.Metadata))
	}
	res.PaymentPerPeriod = h.convertPrice(ctx, core.Price{
		Currency: core.Currency{Type: core.CurrencyTON},
		Amount:   *big.NewInt(sub.PaymentPerPeriod),
	})
	return res
}

func (h *Handler) convertSubscriptionsV1(ctx context.Context, sub core.SubscriptionV1) oas.Subscription {
	res := oas.Subscription{
		Type:           "v1",
		Period:         sub.Period,
		SubscriptionID: fmt.Sprintf("%d", sub.SubscriptionID),
		Wallet:         convertAccountAddress(sub.WalletAccountID, h.addressBook),
	}
	now := time.Now().Unix()
	timeslot := max((now-sub.StartTime)/sub.Period, 0)
	res.NextChargeAt = sub.StartTime + sub.Period*timeslot
	res.Address.SetTo(sub.AccountID.ToRaw())
	beneficiary := convertAccountAddress(sub.BeneficiaryAccountID, h.addressBook)
	res.Beneficiary.SetTo(beneficiary)
	res.Admin.SetTo(beneficiary)
	res.Status = oas.SubscriptionStatusCancelled
	if sub.Status == tlb.AccountActive {
		res.Status = oas.SubscriptionStatusActive
	}
	res.PaymentPerPeriod = h.convertPrice(ctx, core.Price{
		Currency: core.Currency{Type: core.CurrencyTON},
		Amount:   *big.NewInt(sub.Amount),
	})
	return res
}
