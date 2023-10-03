package sse

import (
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/pusher/errors"
	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
	"github.com/tonkeeper/tongo"
)

// Handler handles http methods for sse.
type Handler struct {
	txSource       sources.TransactionSource
	traceSource    sources.TraceSource
	memPool        sources.MemPoolSource
	currentEventID int64
}

type handlerFunc func(session *session, request *http.Request) error

func NewHandler(txSource sources.TransactionSource, traceSource sources.TraceSource, memPool sources.MemPoolSource) *Handler {
	h := Handler{
		txSource:       txSource,
		traceSource:    traceSource,
		memPool:        memPool,
		currentEventID: time.Now().UnixNano(),
	}
	return &h
}

func parseAccounts(str string) (*sources.SubscribeToTransactionsOptions, error) {
	if strings.ToUpper(str) == "ALL" {
		return &sources.SubscribeToTransactionsOptions{AllAccounts: true}, nil
	}
	accountStrings := strings.Split(str, ",")
	accounts := make([]tongo.AccountID, 0, len(accountStrings))
	for _, account := range accountStrings {
		accountID, err := tongo.ParseAccountID(account)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, accountID)
	}
	return &sources.SubscribeToTransactionsOptions{Accounts: accounts}, nil
}

func (h *Handler) SubscribeToTransactions(session *session, request *http.Request) error {
	accounts := request.URL.Query().Get("accounts")
	options, err := parseAccounts(accounts)
	if err != nil {
		return errors.BadRequest("failed to parse 'accounts' parameter in query")
	}
	cancelFn := h.txSource.SubscribeToTransactions(request.Context(), func(data []byte) {
		event := Event{
			Name:    events.AccountTxEvent,
			EventID: h.nextID(),
			Data:    data,
		}
		session.SendEvent(event)
	}, *options)
	session.SetCancelFn(cancelFn)
	return nil
}

func (h *Handler) SubscribeToMessages(session *session, request *http.Request) error {
	cancelFn, err := h.memPool.SubscribeToMessages(request.Context(), func(data []byte) {
		event := Event{
			Name:    events.MempoolEvent,
			EventID: h.nextID(),
			Data:    data,
		}
		session.SendEvent(event)
	})
	if err != nil {
		return err
	}
	session.SetCancelFn(cancelFn)
	return nil
}

func parseAccountsToTraceOptions(str string) (*sources.SubscribeToTraceOptions, error) {
	if strings.ToUpper(str) == "ALL" {
		return &sources.SubscribeToTraceOptions{AllAccounts: true}, nil
	}
	accountStrings := strings.Split(str, ",")
	accounts := make([]tongo.AccountID, 0, len(accountStrings))
	for _, account := range accountStrings {
		accountID, err := tongo.ParseAccountID(account)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, accountID)
	}
	return &sources.SubscribeToTraceOptions{Accounts: accounts}, nil
}

func (h *Handler) SubscribeToTraces(session *session, request *http.Request) error {
	accounts := request.URL.Query().Get("accounts")
	options, err := parseAccountsToTraceOptions(accounts)
	if err != nil {
		return errors.BadRequest("failed to parse 'accounts' parameter in query")
	}
	cancelFn := h.traceSource.SubscribeToTraces(request.Context(), func(data []byte) {
		event := Event{
			Name:    events.TraceEvent,
			EventID: h.nextID(),
			Data:    data,
		}
		session.SendEvent(event)
	}, *options)
	session.SetCancelFn(cancelFn)
	return nil
}

func (h *Handler) nextID() int64 {
	return atomic.AddInt64(&h.currentEventID, 1)
}
