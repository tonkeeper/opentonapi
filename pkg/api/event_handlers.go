package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tonkeeper/tongo/abi"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/sentry"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/tontest"
	"github.com/tonkeeper/tongo/txemulator"
)

var (
	mempoolBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "mempool_messages_batch_size",
		Help:    "Sizes of mempool batches",
		Buckets: []float64{2, 3, 4, 5, 6, 7, 8, 9, 10},
	})
	sendMessageCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tonapi_send_message_counter",
		Help: "The total number of messages received by /v2/blockchain/message endpoint",
	})
	savedEmulatedTraces = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "saved_emulated_traces",
	}, []string{"status"})
)

type decodedMessage struct {
	base64  string
	payload []byte
}

func decodeMessage(s string) (*decodedMessage, error) {
	payload, err := hex.DecodeString(s)
	if err != nil {
		payload, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("boc must be a base64 encoded string or hex string"))
		}
		return &decodedMessage{
			base64:  s,
			payload: payload,
		}, nil
	}
	return &decodedMessage{
		base64:  base64.StdEncoding.EncodeToString(payload),
		payload: payload,
	}, nil
}

func (h *Handler) SendBlockchainMessage(ctx context.Context, request *oas.SendBlockchainMessageReq) error {
	if h.msgSender == nil {
		return toError(http.StatusBadRequest, fmt.Errorf("msg sender is not configured"))
	}
	if !request.Boc.IsSet() && len(request.Batch) == 0 {
		return toError(http.StatusBadRequest, fmt.Errorf("boc not found"))
	}
	var meta map[string]string
	if request.Meta.IsSet() {
		meta = request.Meta.Value
	}
	if request.Boc.IsSet() {
		m, err := decodeMessage(request.Boc.Value)
		if err != nil {
			return err
		}
		checksum := sha256.Sum256(m.payload)
		if _, prs := h.blacklistedBocCache.Get(checksum); prs {
			return toError(http.StatusBadRequest, fmt.Errorf("duplicate message"))
		}
		msgCopy := blockchain.ExtInMsgCopy{
			MsgBoc:  m.base64,
			Payload: m.payload,
			Details: h.ctxToDetails(ctx),
			Meta:    meta,
		}
		sendMessageCounter.Inc()
		if err := h.msgSender.SendMessage(ctx, msgCopy); err != nil {
			if strings.Contains(err.Error(), "cannot apply external message to current state") {
				h.blacklistedBocCache.Set(checksum, struct{}{}, cache.WithExpiration(time.Minute))
				return toError(http.StatusNotAcceptable, err)
			}
			sentry.Send("sending message", sentry.SentryInfoData{"payload": request.Boc}, sentry.LevelError)
			return toError(http.StatusInternalServerError, err)
		}
		h.blacklistedBocCache.Set(checksum, struct{}{}, cache.WithExpiration(time.Minute))
		return nil
	}
	copies := make([]blockchain.ExtInMsgCopy, 0, len(request.Batch))
	for _, msgBoc := range request.Batch {
		m, err := decodeMessage(msgBoc)
		if err != nil {
			return err
		}
		msgCopy := blockchain.ExtInMsgCopy{
			MsgBoc:  m.base64,
			Payload: m.payload,
			Details: h.ctxToDetails(ctx),
			Meta:    meta,
		}
		copies = append(copies, msgCopy)
	}

	sendMessageCounter.Add(float64(len(copies)))
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
	trace, _, _, err = h.storage.GetTraceWithState(ctx, hash.Hex())
	if err == nil && trace != nil {
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
	if errors.Is(err, core.ErrTraceIsTooLong) {
		return nil, toError(http.StatusRequestEntityTooLarge, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if trace.InProgress() {
		traceId := trace.Hash.Hex()[:32] //uuid like hash
		traceEmulated, _, _, err := h.storage.GetTraceWithState(ctx, traceId)
		if err != nil {
			h.logger.Warn("get trace from storage: ", zap.Error(err))
		}
		if traceEmulated != nil {
			traceEmulated = core.CopyTraceData(ctx, trace, traceEmulated)
			trace = traceEmulated
		}
	}
	convertedTrace := convertTrace(trace, h.addressBook)
	if emulated {
		setRecursiveEmulated(&convertedTrace)
	}
	return &convertedTrace, nil
}

func setRecursiveEmulated(trace *oas.Trace) {
	trace.Emulated.SetTo(true)
	for _, c := range trace.Children {
		setRecursiveEmulated(&c)
	}
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
	if errors.Is(err, core.ErrTraceIsTooLong) {
		return nil, toError(http.StatusRequestEntityTooLarge, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	if trace.InProgress() {
		hash := trace.Hash.Hex()[:32] //uuid like hash
		traceEmulated, _, _, err := h.storage.GetTraceWithState(ctx, hash)
		if err != nil {
			h.logger.Warn("get trace from storage: ", zap.Error(err))
		}
		if traceEmulated != nil {
			// we copy data from finished transactions. for emulated it's provided while emulation
			traceEmulated = core.CopyTraceData(ctx, trace, traceEmulated)
			trace = traceEmulated
		}
	}

	actions, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := bath.EnrichWithIntentions(trace, actions)
	event, err := h.toEvent(ctx, trace, result, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	isBannedTraces, err := h.spamFilter.GetEventsScamData(ctx, []string{traceID.Hex()})
	if err != nil {
		h.logger.Warn("error getting events spam data", zap.Error(err))
	}
	event.IsScam = event.IsScam || isBannedTraces[traceID.Hex()]
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

	events := make([]oas.AccountEvent, 0, len(traceIDs))

	var wg sync.WaitGroup
	wg.Add(1)

	var isBannedTraces map[string]bool
	go func() {
		defer wg.Done()
		var eventIDs []string
		for _, traceID := range traceIDs {
			eventIDs = append(eventIDs, traceID.Hash.Hex())
		}
		isBannedTraces, err = h.spamFilter.GetEventsScamData(ctx, eventIDs)
		if err != nil {
			h.logger.Warn("error getting events spam data", zap.Error(err))
		}
	}()

	var lastLT uint64
	for _, traceID := range traceIDs {
		lastLT = traceID.Lt
		trace, err := h.storage.GetTrace(ctx, traceID.Hash)
		if err != nil {
			if errors.Is(err, core.ErrTraceIsTooLong) {
				events = append(events, h.toAccountEventForLongTrace(account.ID, traceID))
			} else {
				events = append(events, h.toUnknownAccountEvent(account.ID, traceID))
			}
			continue
		}
		actions, err := bath.FindActions(ctx, trace, bath.ForAccount(account.ID), bath.WithInformationSource(h.storage))
		if err != nil {
			events = append(events, h.toUnknownAccountEvent(account.ID, traceID))
			continue
			//return nil, toError(http.StatusInternalServerError, err)
		}
		result := bath.EnrichWithIntentions(trace, actions)
		e, err := h.toAccountEvent(ctx, account.ID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
		if err != nil {
			events = append(events, h.toUnknownAccountEvent(account.ID, traceID))
			continue
			//return nil, toError(http.StatusInternalServerError, err)
		}
		events = append(events, e)
	}
	if !(params.BeforeLt.IsSet() || params.StartDate.IsSet() || params.EndDate.IsSet() || (len(events) > 0 && events[0].InProgress)) { //if we look into history we don't need to mix mempool
		memTraces, _ := h.mempoolEmulate.accountsTraces.Get(account.ID)
		i := 0
		for _, hash := range memTraces {
			if i > params.Limit-10 { // we want always to save at least 10 real transaction
				break
			}
			tx, _ := h.storage.SearchTransactionByMessageHash(ctx, hash)
			var trace *core.Trace
			if tx != nil {
				if slices.ContainsFunc(traceIDs, func(t core.TraceID) bool { //скипаем трейсы которые уже есть в ответе
					return t.Hash == *tx
				}) {
					continue
				}
				// try by txHash
				traceId := tx.Hex()[:32] //cuted for uuid
				traceEmulated, _, _, err := h.storage.GetTraceWithState(ctx, traceId)
				if err != nil {
					h.logger.Warn("get trace from storage: ", zap.Error(err))
				}
				if traceEmulated != nil {
					trace = traceEmulated
				}
			}
			if trace == nil {
				// try by external message hash
				traceEmulatedByExternal, _, _, err := h.storage.GetTraceWithState(ctx, hash.Hex())
				if err != nil {
					h.logger.Warn("get trace from storage: ", zap.Error(err))
				}
				if traceEmulatedByExternal != nil {
					trace = traceEmulatedByExternal
				}
			}
			if trace == nil {
				continue
			}
			i++
			actions, err := bath.FindActions(ctx, trace, bath.ForAccount(account.ID), bath.WithInformationSource(h.storage))
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			result := bath.EnrichWithIntentions(trace, actions)
			event, err := h.toAccountEvent(ctx, account.ID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			event.InProgress = true
			event.EventID = hash.Hex()
			events = slices.Insert(events, 0, event)
			if len(events) > params.Limit {
				events = events[:params.Limit]
			}
			lastLT = uint64(events[len(events)-1].Lt)
		}
	}

	wg.Wait()

	if len(events) < params.Limit {
		lastLT = 0 // dirty hack

		////toncenter fallback. don't require now
		//missedLimit := params.Limit - len(events)
		//missedEvents, _ := h.storage.GetMissedEvents(ctx, account.ID, lastLT, missedLimit)
		//events = append(events, missedEvents...)
		//if len(events) > 0 {
		//	lastLT = uint64(events[len(events)-1].Lt)
		//}
	}
	for i, e := range events {
		if e.InProgress {
			for j, _ := range e.Actions {
				events[i].Actions[j].Status = oas.ActionStatusOk
			}
		}
	}
	for i := range events {
		events[i].IsScam = events[i].IsScam || isBannedTraces[events[i].EventID]
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
	var wg sync.WaitGroup
	wg.Add(1)
	var isBannedTraces map[string]bool
	go func() {
		defer wg.Done()
		isBannedTraces, err = h.spamFilter.GetEventsScamData(ctx, []string{traceID.Hex()})
		if err != nil {
			h.logger.Warn("error getting events spam data", zap.Error(err))
		}
	}()
	trace, emulated, err := h.getTraceByHash(ctx, traceID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if errors.Is(err, core.ErrTraceIsTooLong) {
		return nil, toError(http.StatusRequestEntityTooLarge, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	actions, err := bath.FindActions(ctx, trace, bath.ForAccount(account.ID), bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := bath.EnrichWithIntentions(trace, actions)
	event, err := h.toAccountEvent(ctx, account.ID, trace, result, params.AcceptLanguage, params.SubjectOnly.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	wg.Wait()
	event.IsScam = event.IsScam || isBannedTraces[traceID.Hex()]
	if emulated {
		event.InProgress = true
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
	hash := m.Hash(true).Hex()
	trace, version, _, err := h.storage.GetTraceWithState(ctx, hash)
	if err != nil {
		h.logger.Warn("get trace from storage: ", zap.Error(err))
		savedEmulatedTraces.WithLabelValues("error_restore").Inc()
	}
	if trace == nil || h.tongoVersion == 0 || version > h.tongoVersion {
		if version > h.tongoVersion {
			savedEmulatedTraces.WithLabelValues("expired").Inc()
		}
		configBase64, err := h.storage.TrimmedConfigBase64()
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		options := []txemulator.TraceOption{
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithConfigBase64(configBase64),
			txemulator.WithLimit(1100),
		}
		if params.IgnoreSignatureCheck.Value {
			options = append(options, txemulator.WithIgnoreSignatureDepth(1000000))
		}
		emulator, err := txemulator.NewTraceBuilder(options...)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		tree, emulationErr := emulator.Run(ctx, m)
		if emulationErr != nil {
			return nil, toProperEmulationError(emulationErr)
		}
		trace, err = EmulatedTreeToTrace(ctx, h.executor, h.storage, tree, emulator.FinalStates(), nil, h.configPool, true)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		err = h.storage.SaveTraceWithState(ctx, hash, trace, h.tongoVersion, []abi.MethodInvocation{}, 24*time.Hour)
		if err != nil {
			h.logger.Warn("trace not saved: ", zap.Error(err))
			savedEmulatedTraces.WithLabelValues("error_save").Inc()
		}
		savedEmulatedTraces.WithLabelValues("success").Inc()
	} else {
		savedEmulatedTraces.WithLabelValues("restored").Inc()
	}
	actions, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := bath.EnrichWithIntentions(trace, actions)
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
	hs := m.Hash(true).Hex()
	trace, version, _, err := h.storage.GetTraceWithState(ctx, hs)
	if err != nil {
		h.logger.Warn("get trace from storage: ", zap.Error(err))
		savedEmulatedTraces.WithLabelValues("error_restore").Inc()
	}
	if trace == nil || h.tongoVersion == 0 || version > h.tongoVersion {
		if version > h.tongoVersion {
			savedEmulatedTraces.WithLabelValues("expired").Inc()
		}
		configBase64, err := h.storage.TrimmedConfigBase64()
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		options := []txemulator.TraceOption{
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithConfigBase64(configBase64),
		}
		if params.IgnoreSignatureCheck.Value {
			options = append(options, txemulator.WithIgnoreSignatureDepth(1000000))
		}
		emulator, err := txemulator.NewTraceBuilder(options...)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		tree, emulationErr := emulator.Run(ctx, m)
		if emulationErr != nil {
			return nil, toProperEmulationError(emulationErr)
		}
		trace, err = EmulatedTreeToTrace(ctx, h.executor, h.storage, tree, emulator.FinalStates(), nil, h.configPool, true)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		err = h.storage.SaveTraceWithState(ctx, hs, trace, h.tongoVersion, []abi.MethodInvocation{}, 24*time.Hour)
		if err != nil {
			h.logger.Warn("trace not saved: ", zap.Error(err))
			savedEmulatedTraces.WithLabelValues("error_save").Inc()
		}
		savedEmulatedTraces.WithLabelValues("success").Inc()
	} else {
		savedEmulatedTraces.WithLabelValues("restored").Inc()
	}

	actions, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := bath.EnrichWithIntentions(trace, actions)
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
	hs := m.Hash(true).Hex()
	trace, version, _, err := h.storage.GetTraceWithState(ctx, hs)
	if err != nil {
		h.logger.Warn("get trace from storage: ", zap.Error(err))
		savedEmulatedTraces.WithLabelValues("error_restore").Inc()
	}
	if trace == nil || h.tongoVersion == 0 || version > h.tongoVersion {
		if version > h.tongoVersion {
			savedEmulatedTraces.WithLabelValues("expired").Inc()
		}
		configBase64, err := h.storage.TrimmedConfigBase64()
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		options := []txemulator.TraceOption{
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithConfigBase64(configBase64),
		}
		if params.IgnoreSignatureCheck.Value {
			options = append(options, txemulator.WithIgnoreSignatureDepth(1000000))
		}
		emulator, err := txemulator.NewTraceBuilder(options...)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		tree, emulationErr := emulator.Run(ctx, m)
		if emulationErr != nil {
			return nil, toProperEmulationError(emulationErr)
		}
		trace, err = EmulatedTreeToTrace(ctx, h.executor, h.storage, tree, emulator.FinalStates(), nil, h.configPool, true)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		err = h.storage.SaveTraceWithState(ctx, hs, trace, h.tongoVersion, []abi.MethodInvocation{}, 24*time.Hour)
		if err != nil {
			h.logger.Warn("trace not saved: ", zap.Error(err))
			savedEmulatedTraces.WithLabelValues("error_save").Inc()
		}
		savedEmulatedTraces.WithLabelValues("success").Inc()
	} else {
		savedEmulatedTraces.WithLabelValues("restored").Inc()
	}

	t := convertTrace(trace, h.addressBook)
	return &t, nil
}

func extractDestinationWallet(message tlb.Message) (*ton.AccountID, error) {
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
		state.Account.Account.StorageStat.StorageExtra.SumType = "StorageExtraNone"
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
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s GetRawAccount err: %w", walletAddress.ToRaw(), err))
	}
	walletVersion, err := wallet.GetVersionByCode(code)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	risk, err := wallet.ExtractRisk(walletVersion, msgCell)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s GetVersionByCode err: %w", walletAddress.ToRaw(), err))
	}

	hash := m.Hash(true).Hex()
	trace, version, _, err := h.storage.GetTraceWithState(ctx, hash)
	if err != nil {
		h.logger.Warn("get trace from storage: ", zap.Error(err))
		savedEmulatedTraces.WithLabelValues("error_restore").Inc()
	}
	if trace == nil || h.tongoVersion == 0 || version > h.tongoVersion {
		if version > h.tongoVersion {
			savedEmulatedTraces.WithLabelValues("expired").Inc()
		}
		configBase64, err := h.storage.TrimmedConfigBase64()
		if err != nil {
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s TrimmedConfigBase64 err: %w", walletAddress.ToRaw(), err))
		}

		options := []txemulator.TraceOption{
			txemulator.WithConfigBase64(configBase64),
			txemulator.WithAccountsSource(h.storage),
			txemulator.WithLimit(1100),
			txemulator.WithIgnoreSignatureDepth(1),
		}
		accounts, err := convertEmulationParameters(request.Params)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		var states []tlb.ShardAccount
		for accountID, balance := range accounts {
			originalState, err := h.storage.GetAccountState(ctx, accountID)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s GetAccountState err: %w", walletAddress.ToRaw(), err))
			}
			state, err := prepareAccountState(*walletAddress, originalState, balance)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s prepareAccountState err: %w", walletAddress.ToRaw(), err))
			}
			states = append(states, state)
		}

		options = append(options, txemulator.WithAccounts(states...))
		emulator, err := txemulator.NewTraceBuilder(options...)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s NewTraceBuilder err: %w", walletAddress.ToRaw(), err))
		}
		tree, emulationErr := emulator.Run(ctx, m)
		if emulationErr != nil {
			if saveErr := h.storage.SaveEmulationError(ctx, msgCell, hash, emulationErr); saveErr != nil {
				h.logger.Warn("failure to save emulation error: ", zap.Error(saveErr))
			}
			return nil, toProperEmulationError(emulationErr)
		}
		trace, err = EmulatedTreeToTrace(ctx, h.executor, h.storage, tree, emulator.FinalStates(), nil, h.configPool, true)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s EmulatedTreeToTrace err: %w", walletAddress.ToRaw(), err))
		}
		err = h.storage.SaveTraceWithState(ctx, hash, trace, h.tongoVersion, []abi.MethodInvocation{}, 24*time.Hour)
		if err != nil {
			h.logger.Warn("trace not saved: ", zap.Error(err))
			savedEmulatedTraces.WithLabelValues("error_save").Inc()
		}
		savedEmulatedTraces.WithLabelValues("success").Inc()
	} else {
		savedEmulatedTraces.WithLabelValues("restored").Inc()
	}
	t := convertTrace(trace, h.addressBook)
	actions, err := bath.FindActions(ctx, trace, bath.ForAccount(*walletAddress), bath.WithInformationSource(h.storage))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s FindActions err: %w", walletAddress.ToRaw(), err))
	}
	result := bath.EnrichWithIntentions(trace, actions)
	event, err := h.toAccountEvent(ctx, *walletAddress, trace, result, params.AcceptLanguage, true)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s toAccountEvent err: %w", walletAddress.ToRaw(), err))
	}
	oasRisk, err := h.convertRisk(ctx, *risk, *walletAddress)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("account: %s convertRisk err: %w", walletAddress.ToRaw(), err))
	}
	consequences := oas.MessageConsequences{
		Trace: t,
		Event: event,
		Risk:  oasRisk,
	}
	return &consequences, nil
}
