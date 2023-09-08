package api

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/txemulator"
	tongoWallet "github.com/tonkeeper/tongo/wallet"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/sentry"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
)

func (h Handler) SendBlockchainMessage(ctx context.Context, request *oas.SendBlockchainMessageReq) error {
	if h.msgSender == nil {
		return toError(http.StatusBadRequest, fmt.Errorf("msg sender is not configured"))
	}
	if !request.Boc.IsSet() && len(request.Batch) == 0 {
		return toError(http.StatusBadRequest, fmt.Errorf("boc not found"))
	}
	if request.Boc.IsSet() {
		payload, err := sendMessage(ctx, request.Boc.Value, h.msgSender)
		if err != nil {
			sentry.Send("sending message", sentry.SentryInfoData{"payload": request.Boc}, sentry.LevelError)
			return toError(http.StatusInternalServerError, err)
		}
		go func() {
			defer func() {
				if err := recover(); err != nil {
					sentry.Send("addToMempool", sentry.SentryInfoData{"payload": request.Boc}, sentry.LevelError)
				}
			}()
			h.addToMempool(payload, nil)
		}()
	}
	var (
		batchOfBoc   []string
		shardAccount = map[tongo.AccountID]tlb.ShardAccount{}
	)
	for _, msgBoc := range request.Batch {
		payload, err := base64.StdEncoding.DecodeString(msgBoc)
		if err != nil {
			return toError(http.StatusBadRequest, err)
		}
		shardAccount, err = h.addToMempool(payload, shardAccount)
		if err != nil {
			continue
		}
		batchOfBoc = append(batchOfBoc, msgBoc)
	}
	h.msgSender.MsgsBocAddToMempool(batchOfBoc)
	return nil
}

func (h Handler) getTraceByHash(ctx context.Context, hash tongo.Bits256) (*core.Trace, error) {
	trace, err := h.storage.GetTrace(ctx, hash)
	if err == nil || !errors.Is(err, core.ErrEntityNotFound) {
		return trace, err
	}
	txHash, err := h.storage.SearchTransactionByMessageHash(ctx, hash)
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return nil, err
	}
	if err == nil {
		return h.storage.GetTrace(ctx, *txHash)
	}
	trace, ok := h.mempoolEmulate.traces.Get(hash.Hex())
	if ok {
		return trace, nil
	}
	return nil, core.ErrEntityNotFound
}

func (h Handler) GetTrace(ctx context.Context, params oas.GetTraceParams) (*oas.Trace, error) {
	hash, err := tongo.ParseHash(params.TraceID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	trace, err := h.getTraceByHash(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	convertedTrace := convertTrace(*trace, h.addressBook)
	return &convertedTrace, nil
}

func (h Handler) GetEvent(ctx context.Context, params oas.GetEventParams) (*oas.Event, error) {
	traceID, err := tongo.ParseHash(params.EventID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	trace, err := h.getTraceByHash(ctx, traceID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	event, err := h.toEvent(ctx, trace, result, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &event, nil
}

func (h Handler) GetAccountEvents(ctx context.Context, params oas.GetAccountEventsParams) (*oas.AccountEvents, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.SearchTraces(ctx, accountID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusInternalServerError, err)
	}

	events := make([]oas.AccountEvent, len(traceIDs))

	var lastLT uint64
	for i, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		result, err := bath.FindActions(ctx, trace, bath.ForAccount(accountID), bath.WithInformationSource(h.storage))
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		events[i], err = h.toAccountEvent(ctx, accountID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		lastLT = trace.Lt
	}
	if !params.BeforeLt.IsSet() {
		memHashTraces, _ := h.mempoolEmulate.accountsTraces.Get(accountID)
		parsedHashes := make(map[tongo.Bits256]bool)
		for _, traceHash := range memHashTraces {
			parsedHash, _ := tongo.ParseHash(traceHash)
			parsedHashes[parsedHash] = true
		}
		for hash := range parsedHashes {
			_, err = h.storage.SearchTransactionByMessageHash(ctx, hash)
			if err == nil {
				delete(parsedHashes, hash)
			}
		}
		for traceHash := range parsedHashes {
			trace, ok := h.mempoolEmulate.traces.Get(traceHash.Hex())
			if !ok {
				continue
			}
			result, err := bath.FindActions(ctx, trace, bath.ForAccount(accountID), bath.WithInformationSource(h.storage))
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			event, err := h.toAccountEvent(ctx, accountID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			event.InProgress = true
			event.EventID = traceHash.Hex()
			events = slices.Insert(events, 0, event)[:len(events)-1]
			lastLT = uint64(events[len(events)-1].Lt)
		}
	}
	for _, event := range events {
		for i, j := 0, len(event.Actions)-1; i < j; i, j = i+1, j-1 {
			event.Actions[i], event.Actions[j] = event.Actions[j], event.Actions[i]
		}
	}
	return &oas.AccountEvents{Events: events, NextFrom: int64(lastLT)}, nil
}

func (h Handler) GetAccountEvent(ctx context.Context, params oas.GetAccountEventParams) (*oas.AccountEvent, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceID, err := tongo.ParseHash(params.EventID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	trace, err := h.storage.GetTrace(ctx, traceID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result, err := bath.FindActions(ctx, trace, bath.ForAccount(accountID), bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	event, err := h.toAccountEvent(ctx, accountID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for i, j := 0, len(event.Actions)-1; i < j; i, j = i+1, j-1 {
		event.Actions[i], event.Actions[j] = event.Actions[j], event.Actions[i]
	}
	return &event, nil
}

func (h Handler) EmulateMessageToAccountEvent(ctx context.Context, request *oas.EmulateMessageToAccountEventReq, params oas.EmulateMessageToAccountEventParams) (*oas.AccountEvent, error) {
	c, err := boc.DeserializeSinglRootBase64(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	emulator, err := txemulator.NewTraceBuilder(
		txemulator.WithAccountsSource(h.storage),
		//txemulator.WithSignatureCheck(), todo: uncomment after tonspace switch to /v2/eallet/emulate
		txemulator.WithConfigBase64(configBase64))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.storage, configBase64, tree, emulator.FinalStates())
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	event, err := h.toAccountEvent(ctx, account, trace, result, params.AcceptLanguage, false)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &event, nil
}

func (h Handler) EmulateMessageToEvent(ctx context.Context, request *oas.EmulateMessageToEventReq, params oas.EmulateMessageToEventParams) (*oas.Event, error) {
	c, err := boc.DeserializeSinglRootBase64(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	emulator, err := txemulator.NewTraceBuilder(
		txemulator.WithAccountsSource(h.storage),
		txemulator.WithSignatureCheck(),
		txemulator.WithConfigBase64(configBase64))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.storage, configBase64, tree, emulator.FinalStates())
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	event, err := h.toEvent(ctx, trace, result, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &event, nil
}

func (h Handler) EmulateMessageToTrace(ctx context.Context, request *oas.EmulateMessageToTraceReq) (*oas.Trace, error) {
	c, err := boc.DeserializeSinglRootBase64(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	emulator, err := txemulator.NewTraceBuilder(
		txemulator.WithAccountsSource(h.storage),
		txemulator.WithSignatureCheck(),
		txemulator.WithConfigBase64(configBase64))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.storage, configBase64, tree, emulator.FinalStates())
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
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

func (h Handler) EmulateMessageToWallet(ctx context.Context, request *oas.EmulateMessageToWalletReq, params oas.EmulateMessageToWalletParams) (*oas.MessageConsequences, error) {
	msgCell, err := boc.DeserializeSinglRootBase64(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	err = tlb.Unmarshal(msgCell, &m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	walletAddress, err := extractDestinationWallet(m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	account, err := h.storage.GetRawAccount(ctx, *walletAddress)
	if err != nil {
		// TODO: if not found, take code from stateInit
		return nil, toError(http.StatusInternalServerError, err)
	}
	walletVersion, err := wallet.GetVersionByCode(account.Code)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	risk, err := wallet.ExtractRisk(walletVersion, msgCell)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	emulator, err := txemulator.NewTraceBuilder(
		txemulator.WithAccountsSource(h.storage),
		txemulator.WithConfigBase64(configBase64))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.storage, configBase64, tree, emulator.FinalStates())
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	t := convertTrace(*trace, h.addressBook)
	result, err := bath.FindActions(ctx, trace, bath.ForAccount(*walletAddress), bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	event, err := h.toAccountEvent(ctx, *walletAddress, trace, result, params.AcceptLanguage, true)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	oasRisk, err := h.convertRisk(ctx, *risk, *walletAddress)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	consequences := oas.MessageConsequences{
		Trace: t,
		Event: event,
		Risk:  oasRisk,
	}
	return &consequences, nil
}

func (h Handler) addToMempool(bytesBoc []byte, shardAccount map[tongo.AccountID]tlb.ShardAccount) (map[tongo.AccountID]tlb.ShardAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if shardAccount == nil {
		shardAccount = map[tongo.AccountID]tlb.ShardAccount{}
	}
	msgCell, err := boc.DeserializeBoc(bytesBoc)
	if err != nil {
		return shardAccount, err
	}
	ttl := int64(30)
	msgV4, err := tongoWallet.DecodeMessageV4(msgCell[0])
	if err == nil {
		ttl = int64(msgV4.ValidUntil) - time.Now().Unix()
	}
	var message tlb.Message
	err = tlb.Unmarshal(msgCell[0], &message)
	if err != nil {
		return shardAccount, err
	}
	config, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	emulator, err := txemulator.NewTraceBuilder(txemulator.WithAccountsSource(h.storage),
		txemulator.WithAccountsMap(shardAccount),
		txemulator.WithConfigBase64(config))
	if err != nil {
		return shardAccount, err
	}
	tree, err := emulator.Run(ctx, message)
	if err != nil {
		return shardAccount, err
	}
	newShardAccount := emulator.FinalStates()
	trace, err := emulatedTreeToTrace(ctx, h.storage, config, tree, newShardAccount)
	if err != nil {
		return shardAccount, err
	}
	accounts := make(map[tongo.AccountID]struct{})
	var traverse func(*core.Trace)
	traverse = func(node *core.Trace) {
		accounts[node.Account] = struct{}{}
		for _, child := range node.Children {
			traverse(child)
		}
	}
	traverse(trace)
	hash, err := msgCell[0].Hash()
	if err != nil {
		return shardAccount, err
	}
	h.mempoolEmulate.traces.Set(hex.EncodeToString(hash), trace, cache.WithExpiration(time.Second*time.Duration(ttl)))
	h.mempoolEmulate.mu.Lock()
	defer h.mempoolEmulate.mu.Unlock()
	for account := range accounts {
		traces, _ := h.mempoolEmulate.accountsTraces.Get(account)
		traces = slices.Insert(traces, 0, hex.EncodeToString(hash))
		h.mempoolEmulate.accountsTraces.Set(account, traces, cache.WithExpiration(time.Second*time.Duration(ttl)))
	}
	return newShardAccount, nil
}

func emulatedTreeToTrace(ctx context.Context, resolver core.LibraryResolver, configBase64 string, tree *txemulator.TxTree, accounts map[tongo.AccountID]tlb.ShardAccount) (*core.Trace, error) {
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
		Transaction:    *transaction,
		AdditionalInfo: &core.TraceAdditionalInfo{},
	}
	for i := range tree.Children {
		child, err := emulatedTreeToTrace(ctx, resolver, configBase64, tree.Children[i], accounts)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, child)
	}
	executor := newSharedAccountExecutor(accounts, resolver, configBase64)
	for k, v := range accounts {
		code := accountCode(v)
		if code == nil {
			continue
		}
		b, err := code.ToBoc()
		if err != nil {
			return nil, err
		}
		inspectionResult, err := abi.NewContractInspector().InspectContract(ctx, b, executor, k)
		if err != nil {
			return nil, err
		}
		t.AccountInterfaces = inspectionResult.ImplementedInterfaces()
		for _, i := range inspectionResult.Interfaces {
			for _, m := range i.GetMethods {
				switch data := m.Result.(type) {
				case abi.GetWalletDataResult:
					t.AdditionalInfo.JettonMaster, _ = tongo.AccountIDFromTlb(data.Jetton)
				case abi.GetSaleData_GetgemsResult:
					price := big.Int(data.FullPrice)
					owner, err := tongo.AccountIDFromTlb(data.Owner)
					if err != nil {
						continue
					}
					t.AdditionalInfo.NftSaleContract = &core.NftSaleContract{
						NftPrice: price.Int64(),
						Owner:    owner,
					}
				case abi.GetSaleData_BasicResult:
					price := big.Int(data.FullPrice)
					owner, err := tongo.AccountIDFromTlb(data.Owner)
					if err != nil {
						continue
					}
					t.AdditionalInfo.NftSaleContract = &core.NftSaleContract{
						NftPrice: price.Int64(),
						Owner:    owner,
					}
				case abi.GetSaleData_GetgemsAuctionResult:
					owner, err := tongo.AccountIDFromTlb(data.Owner)
					if err != nil {
						continue
					}
					t.AdditionalInfo.NftSaleContract = &core.NftSaleContract{
						NftPrice: int64(data.MaxBid),
						Owner:    owner,
					}
				case abi.GetPoolData_StonfiResult:
					t0, err0 := tongo.AccountIDFromTlb(data.Token0Address)
					t1, err1 := tongo.AccountIDFromTlb(data.Token1Address)
					if err1 != nil || err0 != nil {
						continue
					}
					t.AdditionalInfo.STONfiPool = &core.STONfiPool{
						Token0: *t0,
						Token1: *t1,
					}
				}
			}
		}
	}
	return t, nil
}

func sendMessage(ctx context.Context, msgBoc string, msgSender messageSender) ([]byte, error) {
	payload, err := base64.StdEncoding.DecodeString(msgBoc)
	if err != nil {
		return nil, err
	}
	err = msgSender.SendMessage(ctx, payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
