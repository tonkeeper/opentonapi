package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/tontest"
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

const maxBatchSize = 5

var (
	mempoolBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "mempool_messages_batch_size",
		Help:    "Sizes of mempool batches",
		Buckets: []float64{2, 3, 4, 5, 6, 7, 8, 9, 10},
	})
	mempoolMessageCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mempool_messages_counter",
		Help: "The total number of mempool messages",
	})
)

func (h *Handler) SendBlockchainMessage(ctx context.Context, request *oas.SendBlockchainMessageReq) error {
	if h.msgSender == nil {
		return toError(http.StatusBadRequest, fmt.Errorf("msg sender is not configured"))
	}
	if !request.Boc.IsSet() && len(request.Batch) == 0 {
		return toError(http.StatusBadRequest, fmt.Errorf("boc not found"))
	}
	if request.Boc.IsSet() {
		payload, err := base64.StdEncoding.DecodeString(request.Boc.Value)
		if err != nil {
			return toError(http.StatusBadRequest, fmt.Errorf("boc must be a base64 encoded string"))
		}
		checksum := sha256.Sum256(payload)
		if _, prs := h.blacklistedBocCache.Get(checksum); prs {
			return toError(http.StatusBadRequest, fmt.Errorf("duplicate message"))
		}
		msgCopy := blockchain.ExtInMsgCopy{
			MsgBoc:  request.Boc.Value,
			Payload: payload,
			Details: h.ctxToDetails(ctx),
		}
		mempoolMessageCounter.Inc()
		if err := h.msgSender.SendMessage(ctx, msgCopy); err != nil {
			if strings.Contains(err.Error(), "cannot apply external message to current state") {
				h.blacklistedBocCache.Set(checksum, struct{}{}, cache.WithExpiration(time.Minute))
				return toError(http.StatusNotAcceptable, err)
			}
			sentry.Send("sending message", sentry.SentryInfoData{"payload": request.Boc}, sentry.LevelError)
			return toError(http.StatusInternalServerError, err)
		}
		h.blacklistedBocCache.Set(checksum, struct{}{}, cache.WithExpiration(time.Minute))
		go func() {
			defer func() {
				if err := recover(); err != nil {
					sentry.Send("addToMempool", sentry.SentryInfoData{"payload": request.Boc}, sentry.LevelError)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			h.addToMempool(ctx, payload, nil)
		}()
		return nil
	}
	var (
		copies       []blockchain.ExtInMsgCopy
		shardAccount = map[tongo.AccountID]tlb.ShardAccount{}
	)
	if len(request.Batch) > maxBatchSize {
		return toError(http.StatusBadRequest, fmt.Errorf("batch size must be less than %v", maxBatchSize))
	}
	for _, msgBoc := range request.Batch {
		payload, err := base64.StdEncoding.DecodeString(msgBoc)
		if err != nil {
			return toError(http.StatusBadRequest, err)
		}
		shardAccount, err = h.addToMempool(ctx, payload, shardAccount)
		if err != nil {
			continue
		}
		msgCopy := blockchain.ExtInMsgCopy{
			MsgBoc:  msgBoc,
			Payload: payload,
			Details: h.ctxToDetails(ctx),
		}
		copies = append(copies, msgCopy)
	}

	mempoolMessageCounter.Add(float64(len(copies)))
	mempoolBatchSize.Observe(float64(len(copies)))

	h.msgSender.SendMultipleMessages(ctx, copies)
	return nil
}

func (h *Handler) getTraceByHash(ctx context.Context, hash tongo.Bits256) (*core.Trace, bool, error) {
	trace, err := h.storage.GetTrace(ctx, hash)
	if err == nil || !errors.Is(err, core.ErrEntityNotFound) {
		return trace, false, err
	}
	txHash, err := h.storage.SearchTransactionByMessageHash(ctx, hash)
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return nil, false, err
	}
	if err == nil {
		trace, err = h.storage.GetTrace(ctx, *txHash)
		return trace, false, err
	}
	trace, ok := h.mempoolEmulate.traces.Get(hash.Hex())
	if ok {
		return trace, true, nil
	}
	return nil, false, core.ErrEntityNotFound
}

func (h *Handler) GetTrace(ctx context.Context, params oas.GetTraceParams) (*oas.Trace, error) {
	hash, err := tongo.ParseHash(params.TraceID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	trace, emulated, err := h.getTraceByHash(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	convertedTrace := convertTrace(*trace, h.addressBook)
	if emulated {
		convertedTrace.Emulated.SetTo(true)
	}
	return &convertedTrace, nil
}

func (h *Handler) GetEvent(ctx context.Context, params oas.GetEventParams) (*oas.Event, error) {
	traceID, err := tongo.ParseHash(params.EventID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	trace, emulated, err := h.getTraceByHash(ctx, traceID)
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
	if emulated {
		event.InProgress = true
	}
	return &event, nil
}

func (h *Handler) GetAccountEvents(ctx context.Context, params oas.GetAccountEventsParams) (*oas.AccountEvents, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.SearchTraces(ctx, account.ID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate), params.Initiator.Value)
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
		result, err := bath.FindActions(ctx, trace, bath.ForAccount(account.ID), bath.WithInformationSource(h.storage))
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		events[i], err = h.toAccountEvent(ctx, account.ID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		lastLT = trace.Lt
	}
	if !params.BeforeLt.IsSet() {
		memHashTraces, _ := h.mempoolEmulate.accountsTraces.Get(account.ID)
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
			result, err := bath.FindActions(ctx, trace, bath.ForAccount(account.ID), bath.WithInformationSource(h.storage))
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			event, err := h.toAccountEvent(ctx, account.ID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			event.InProgress = true
			event.EventID = traceHash.Hex()
			events = slices.Insert(events, 0, event)
			if len(events) > params.Limit {
				events = events[:params.Limit]
			}
			lastLT = uint64(events[len(events)-1].Lt)
		}
	}

	return &oas.AccountEvents{Events: events, NextFrom: int64(lastLT)}, nil
}

func (h *Handler) GetAccountEvent(ctx context.Context, params oas.GetAccountEventParams) (*oas.AccountEvent, error) {
	account, err := tongo.ParseAddress(params.AccountID)
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
	result, err := bath.FindActions(ctx, trace, bath.ForAccount(account.ID), bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	event, err := h.toAccountEvent(ctx, account.ID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for i, j := 0, len(event.Actions)-1; i < j; i, j = i+1, j-1 {
		event.Actions[i], event.Actions[j] = event.Actions[j], event.Actions[i]
	}
	return &event, nil
}

func toProperEmulationError(err error) error {
	if err != nil {
		errWithCode, ok := err.(txemulator.ErrorWithExitCode)
		if ok && errWithCode.Iteration == 0 {
			// we want return Not Acceptable when the destination contract didn't accept a message.
			return toError(http.StatusNotAcceptable, err)
		}
		return toError(http.StatusInternalServerError, err)
	}
	return nil
}

func (h *Handler) EmulateMessageToAccountEvent(ctx context.Context, request *oas.EmulateMessageToAccountEventReq, params oas.EmulateMessageToAccountEventParams) (*oas.AccountEvent, error) {
	c, err := deserializeSingleBoc(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
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
		return nil, toProperEmulationError(err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.executor, h.storage, configBase64, tree, emulator.FinalStates())
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	event, err := h.toAccountEvent(ctx, account.ID, trace, result, params.AcceptLanguage, false)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &event, nil
}

func (h *Handler) EmulateMessageToEvent(ctx context.Context, request *oas.EmulateMessageToEventReq, params oas.EmulateMessageToEventParams) (*oas.Event, error) {
	c, err := deserializeSingleBoc(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	if err := tlb.Unmarshal(c, &m); err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var emulator *txemulator.Tracer
	if params.IgnoreSignatureCheck.Value {
		emulator, err = txemulator.NewTraceBuilder(
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithConfigBase64(configBase64))
	} else {
		emulator, err = txemulator.NewTraceBuilder(
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithSignatureCheck(),
			txemulator.WithConfigBase64(configBase64))
	}

	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return nil, toProperEmulationError(err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.executor, h.storage, configBase64, tree, emulator.FinalStates())
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

func (h *Handler) EmulateMessageToTrace(ctx context.Context, request *oas.EmulateMessageToTraceReq, params oas.EmulateMessageToTraceParams) (*oas.Trace, error) {
	c, err := deserializeSingleBoc(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	err = tlb.Unmarshal(c, &m)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var emulator *txemulator.Tracer
	if params.IgnoreSignatureCheck.Value {
		emulator, err = txemulator.NewTraceBuilder(
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithConfigBase64(configBase64))
	} else {
		emulator, err = txemulator.NewTraceBuilder(
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithSignatureCheck(),
			txemulator.WithConfigBase64(configBase64))
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return nil, toProperEmulationError(err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.executor, h.storage, configBase64, tree, emulator.FinalStates())
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

func prepareAccountState(accountID tongo.AccountID, state tlb.ShardAccount, startBalance int64) (tlb.ShardAccount, error) {
	if state.Account.Status() == tlb.AccountActive {
		state.Account.Account.Storage.Balance.Grams = tlb.Grams(startBalance)
		return state, nil
	}
	return tontest.
		Account().
		Balance(tlb.Grams(startBalance)).
		Address(accountID).
		ShardAccount()
}

func convertEmulationParameters(params []oas.EmulateMessageToWalletReqParamsItem) (map[tongo.AccountID]int64, error) {
	result := make(map[tongo.AccountID]int64, len(params))
	for _, p := range params {
		if !p.GetBalance().IsSet() {
			continue
		}
		balance := p.GetBalance().Value
		if balance < 0 {
			return nil, fmt.Errorf("balance must be greater than 0")
		}
		addr, err := tongo.ParseAddress(p.Address)
		if err != nil {
			return nil, err
		}
		result[addr.ID] = balance
	}
	return result, nil
}

func (h *Handler) EmulateMessageToWallet(ctx context.Context, request *oas.EmulateMessageToWalletReq, params oas.EmulateMessageToWalletParams) (*oas.MessageConsequences, error) {
	msgCell, err := deserializeSingleBoc(request.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var m tlb.Message
	err = tlb.Unmarshal(msgCell, &m)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	walletAddress, err := extractDestinationWallet(m)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var code []byte
	if account, err := h.storage.GetRawAccount(ctx, *walletAddress); err == nil && len(account.Code) > 0 {
		code = account.Code
	} else if m.Init.Exists && m.Init.Value.Value.Code.Exists {
		code, err = m.Init.Value.Value.Code.Value.Value.ToBoc()
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
	} else if err == nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("code not found and message doesn't have init"))
	} else {
		return nil, toError(http.StatusInternalServerError, err)
	}
	walletVersion, err := wallet.GetVersionByCode(code)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	risk, err := wallet.ExtractRisk(walletVersion, msgCell)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	options := []txemulator.TraceOption{
		txemulator.WithConfigBase64(configBase64),
		txemulator.WithAccountsSource(h.storage),
	}
	accounts, err := convertEmulationParameters(request.Params)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var states []tlb.ShardAccount
	for accountID, balance := range accounts {
		originalState, err := h.storage.GetAccountState(ctx, accountID)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		state, err := prepareAccountState(*walletAddress, originalState, balance)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		states = append(states, state)
	}
	options = append(options, txemulator.WithAccounts(states...))
	emulator, err := txemulator.NewTraceBuilder(options...)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	tree, err := emulator.Run(ctx, m)
	if err != nil {
		return nil, toProperEmulationError(err)
	}
	trace, err := emulatedTreeToTrace(ctx, h.executor, h.storage, configBase64, tree, emulator.FinalStates())
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

func (h *Handler) addToMempool(ctx context.Context, bytesBoc []byte, shardAccount map[tongo.AccountID]tlb.ShardAccount) (map[tongo.AccountID]tlb.ShardAccount, error) {
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
		return shardAccount, err
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
	trace, err := emulatedTreeToTrace(ctx, h.executor, h.storage, config, tree, newShardAccount)
	if err != nil {
		return shardAccount, err
	}
	accounts := make(map[tongo.AccountID]struct{})
	core.Visit(trace, func(node *core.Trace) {
		accounts[node.Account] = struct{}{}
	})
	hash, err := msgCell[0].Hash()
	if err != nil {
		return shardAccount, err
	}
	h.mempoolEmulate.traces.Set(hex.EncodeToString(hash), trace, cache.WithExpiration(time.Second*time.Duration(ttl)))
	h.mempoolEmulate.mu.Lock()
	defer h.mempoolEmulate.mu.Unlock()
	for account := range accounts {
		if _, ok := h.mempoolEmulateIgnoreAccounts[account]; ok {
			continue
		}
		traces, _ := h.mempoolEmulate.accountsTraces.Get(account)
		traces = slices.Insert(traces, 0, hex.EncodeToString(hash))
		h.mempoolEmulate.accountsTraces.Set(account, traces, cache.WithExpiration(time.Second*time.Duration(ttl)))
	}
	h.emulationCh <- blockchain.ExtInMsgCopy{
		MsgBoc:   base64.StdEncoding.EncodeToString(bytesBoc),
		Details:  h.ctxToDetails(ctx),
		Payload:  bytesBoc,
		Accounts: accounts,
	}
	return newShardAccount, nil
}

func emulatedTreeToTrace(ctx context.Context, executor executor, resolver core.LibraryResolver, configBase64 string, tree *txemulator.TxTree, accounts map[tongo.AccountID]tlb.ShardAccount) (*core.Trace, error) {
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
	filteredMsgs := make([]core.Message, 0, len(transaction.OutMsgs))
	for _, msg := range transaction.OutMsgs {
		if msg.Destination == nil {
			filteredMsgs = append(filteredMsgs, msg)
		}
	}
	transaction.OutMsgs = filteredMsgs //all internal messages in emulation result are delivered to another account and created transaction
	if err != nil {
		return nil, err
	}
	t := &core.Trace{
		Transaction: *transaction,
	}
	additionalInfo := &core.TraceAdditionalInfo{}
	for i := range tree.Children {
		child, err := emulatedTreeToTrace(ctx, executor, resolver, configBase64, tree.Children[i], accounts)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, child)
	}
	accountID := t.Account
	code := accountCode(accounts[accountID])
	if code == nil {
		return t, nil
	}
	b, err := code.ToBoc()
	if err != nil {
		return nil, err
	}
	sharedExecutor := newSharedAccountExecutor(accounts, executor, resolver, configBase64)
	inspectionResult, err := abi.NewContractInspector().InspectContract(ctx, b, sharedExecutor, accountID)
	if err != nil {
		return nil, err
	}
	implemented := make(map[abi.ContractInterface]struct{}, len(inspectionResult.ContractInterfaces))
	for _, iface := range inspectionResult.ContractInterfaces {
		implemented[iface] = struct{}{}
	}
	// TODO: for all obtained Jetton Masters confirm that jetton wallets are valid
	t.AccountInterfaces = inspectionResult.ContractInterfaces
	for _, m := range inspectionResult.GetMethods {
		switch data := m.Result.(type) {
		case abi.GetNftDataResult:
			if _, ok := implemented[abi.Teleitem]; !ok {
				continue
			}
			value := big.Int(data.Index)
			index := decimal.NewFromBigInt(&value, 0)
			collectionAddr, err := tongo.AccountIDFromTlb(data.CollectionAddress)
			if err != nil || collectionAddr == nil {
				continue
			}
			_, nftByIndex, err := abi.GetNftAddressByIndex(ctx, sharedExecutor, *collectionAddr, data.Index)
			if err != nil {
				continue
			}
			indexResult, ok := nftByIndex.(abi.GetNftAddressByIndexResult)
			if !ok {
				continue
			}
			nftAddr, err := tongo.AccountIDFromTlb(indexResult.Address)
			if err != nil || nftAddr == nil {
				continue
			}
			additionalInfo.EmulatedTeleitemNFT = &core.EmulatedTeleitemNFT{
				Index:             index,
				CollectionAddress: collectionAddr,
				Verified:          *nftAddr == accountID,
			}
		case abi.GetWalletDataResult:
			master, _ := tongo.AccountIDFromTlb(data.Jetton)
			additionalInfo.SetJettonMaster(accountID, *master)
		case abi.GetSaleData_GetgemsResult:
			price := big.Int(data.FullPrice)
			owner, err := tongo.AccountIDFromTlb(data.Owner)
			if err != nil {
				continue
			}
			additionalInfo.NftSaleContract = &core.NftSaleContract{
				NftPrice: price.Int64(),
				Owner:    owner,
			}
		case abi.GetSaleData_BasicResult:
			price := big.Int(data.FullPrice)
			owner, err := tongo.AccountIDFromTlb(data.Owner)
			if err != nil {
				continue
			}
			additionalInfo.NftSaleContract = &core.NftSaleContract{
				NftPrice: price.Int64(),
				Owner:    owner,
			}
		case abi.GetSaleData_GetgemsAuctionResult:
			owner, err := tongo.AccountIDFromTlb(data.Owner)
			if err != nil {
				continue
			}
			additionalInfo.NftSaleContract = &core.NftSaleContract{
				NftPrice: int64(data.MaxBid),
				Owner:    owner,
			}
		case abi.GetPoolData_StonfiResult:
			t0, err0 := tongo.AccountIDFromTlb(data.Token0Address)
			t1, err1 := tongo.AccountIDFromTlb(data.Token1Address)
			if err1 != nil || err0 != nil {
				continue
			}
			additionalInfo.STONfiPool = &core.STONfiPool{
				Token0: *t0,
				Token1: *t1,
			}
			for _, accountID := range []ton.AccountID{*t0, *t1} {
				_, value, err := abi.GetWalletData(ctx, sharedExecutor, accountID)
				if err != nil {
					return nil, err
				}
				data := value.(abi.GetWalletDataResult)
				master, _ := tongo.AccountIDFromTlb(data.Jetton)
				additionalInfo.SetJettonMaster(accountID, *master)
			}
		}
	}
	t.SetAdditionalInfo(additionalInfo)
	return t, nil
}
