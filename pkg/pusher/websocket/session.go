package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tonkeeper/opentonapi/pkg/pusher/events"
	"github.com/tonkeeper/opentonapi/pkg/pusher/metrics"
	"github.com/tonkeeper/opentonapi/pkg/pusher/utils"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
)

const subscriptionLimit = 10000 // limitation of subscription by connection

// session is a light-weight implementation of JSON-RPC protocol over an HTTP connection from a client.
type session struct {
	logger              *zap.Logger
	conn                *websocket.Conn
	mempool             sources.MemPoolSource
	txSource            sources.TransactionSource
	traceSource         sources.TraceSource
	eventCh             chan event
	txSubscriptions     map[tongo.AccountID]sources.CancelFn
	traceSubscriptions  map[tongo.AccountID]sources.CancelFn
	mempoolSubscription sources.CancelFn
	pingInterval        time.Duration
	subscriptionLimit   int
}

type event struct {
	Name   events.Name
	Method string
	Params []byte
}

func newSession(logger *zap.Logger, txSource sources.TransactionSource, traceSource sources.TraceSource, mempool sources.MemPoolSource, conn *websocket.Conn) *session {
	return &session{
		logger:             logger,
		eventCh:            make(chan event, 1000),
		conn:               conn,
		mempool:            mempool,
		txSource:           txSource,
		txSubscriptions:    map[tongo.AccountID]sources.CancelFn{},
		traceSource:        traceSource,
		traceSubscriptions: map[tongo.AccountID]sources.CancelFn{},
		pingInterval:       5 * time.Second,
		subscriptionLimit:  subscriptionLimit,
	}
}

func (s *session) cancel() {
	for _, cancelFn := range s.txSubscriptions {
		cancelFn()
	}
	for _, cancelFn := range s.traceSubscriptions {
		cancelFn()
	}
	if s.mempoolSubscription != nil {
		s.mempoolSubscription()
	}
}

func (s *session) Run(ctx context.Context) chan JsonRPCRequest {
	requestCh := make(chan JsonRPCRequest)
	go func() {
		defer s.cancel()

		for {
			var err error
			select {
			case <-ctx.Done():
				return
			case e := <-s.eventCh:
				response := JsonRPCResponse{
					JSONRPC: "2.0",
					Method:  e.Method,
					Params:  e.Params,
				}
				metrics.WebsocketEventSent(e.Name, utils.TokenNameFromContext(ctx))
				err = s.conn.WriteJSON(response)
			case request := <-requestCh:
				var response string
				switch request.Method {
				// handle transaction subscriptions
				case "subscribe_account":
					response = s.subscribeToTransactions(ctx, request.Params)
				case "unsubscribe_account":
					response = s.unsubscribeFromTransactions(request.Params)

				// handle mempool subscriptions
				case "subscribe_mempool":
					response = s.subscribeToMempool(ctx, request.Params)
				case "unsubscribe_mempool":
					response = s.unsubscribeFromMempool()

				// handle trace subscriptions
				case "subscribe_trace":
					response = s.subscribeToTraces(ctx, request.Params)
				case "unsubscribe_trace":
					response = s.unsubscribeFromTraces(request.Params)
				}
				err = s.writeResponse(response, request)
			case <-time.After(s.pingInterval):
				metrics.WebsocketEventSent(events.PingEvent, utils.TokenNameFromContext(ctx))
				err = s.conn.WriteMessage(websocket.PingMessage, []byte{})
			}
			if err != nil {
				s.logger.Error("websocket session failed", zap.Error(err))
				return
			}
		}
	}()
	return requestCh
}

func (s *session) sendEvent(e event) {
	select {
	case s.eventCh <- e:
	default:
		// TODO: maybe we should either close the channel or let the user know that we have dropped an event
		s.logger.Warn("event channel is full, dropping event",
			zap.String("event", string(e.Name)))
	}
}

type accountOptions struct {
	Account    tongo.AccountID
	Operations []string
}

func (opts *accountOptions) AllOperations() bool {
	return len(opts.Operations) == 0
}

func processAccountTxParam(param string) (*accountOptions, error) {
	parts := strings.Split(param, ";")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid format: '%v'", param)
	}
	if len(parts) == 1 {
		account, err := tongo.ParseAddress(param)
		if err != nil {
			return nil, fmt.Errorf("failed to process '%v' account: %v", param, err)
		}
		return &accountOptions{Account: account.ID}, nil
	}
	if len(parts) > 2 {
		return nil, fmt.Errorf("failed to process '%v' account: invalid format", param)
	}
	account, err := tongo.ParseAddress(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to process '%v' account: %v", param, err)
	}
	opParts := strings.Split(parts[1], "=")
	if len(opParts) != 2 {
		return nil, fmt.Errorf("failed to process '%v' account: invalid format", param)
	}
	if strings.ToLower(opParts[0]) != "operations" {
		return nil, fmt.Errorf("failed to process '%v' account: invalid format", param)
	}
	operations := strings.Split(opParts[1], ",")
	if len(operations) == 0 || len(opParts[1]) == 0 {
		return &accountOptions{Account: account.ID}, nil
	}
	return &accountOptions{Account: account.ID, Operations: operations}, nil
}

// subscribeToTransactions subscribes to transactions for the specified accounts.
// Each param should be in the following format: "<accountID>;operations=<op1>,<op2>,..."
// if there is no ";operations=" part, a given account will be subscribed to all operations.
func (s *session) subscribeToTransactions(ctx context.Context, params []string) string {
	accounts := make(map[tongo.AccountID]accountOptions, len(params))
	for _, param := range params {
		options, err := processAccountTxParam(param)
		if err != nil {
			return err.Error()
		}
		accounts[options.Account] = *options
	}
	if len(s.txSubscriptions)+len(accounts) > s.subscriptionLimit {
		return fmt.Sprintf("you have reached the limit of %v subscriptions", s.subscriptionLimit)
	}
	var counter int
	for account, accountOptions := range accounts {
		if _, ok := s.txSubscriptions[account]; ok {
			continue
		}
		options := sources.SubscribeToTransactionsOptions{
			Accounts:      []tongo.AccountID{account},
			Operations:    accountOptions.Operations,
			AllOperations: accountOptions.AllOperations(),
		}
		cancel := s.txSource.SubscribeToTransactions(ctx, func(eventData []byte) {
			s.sendEvent(event{
				Name:   events.AccountTxEvent,
				Method: "account_transaction",
				Params: eventData,
			})
		}, options)
		s.txSubscriptions[account] = cancel
		counter += 1
	}
	return fmt.Sprintf("success! %v new subscriptions created", counter)
}

func (s *session) unsubscribeFromTransactions(params []string) string {
	var counter int
	for _, a := range params {
		account, err := tongo.ParseAddress(a)
		if err != nil {
			return fmt.Sprintf("failed to process '%v' account: %v", a, err)
		}
		if cancelFn, ok := s.txSubscriptions[account.ID]; ok {
			cancelFn()
			delete(s.txSubscriptions, account.ID)
			counter += 1
		}
	}
	return fmt.Sprintf("success! %v subscription(s) removed", counter)
}

func (s *session) subscribeToTraces(ctx context.Context, params []string) string {
	accounts := make([]tongo.AccountID, 0, len(params))
	for _, a := range params {
		account, err := tongo.ParseAddress(a)
		if err != nil {
			return fmt.Sprintf("failed to process '%v' account: %v", a, err)
		}
		accounts = append(accounts, account.ID)
	}
	if len(s.traceSubscriptions)+len(accounts) > s.subscriptionLimit {
		return fmt.Sprintf("you have reached the limit of %v subscriptions", s.subscriptionLimit)
	}
	var counter int
	for _, account := range accounts {
		if _, ok := s.traceSubscriptions[account]; ok {
			continue
		}
		options := sources.SubscribeToTraceOptions{
			Accounts: []tongo.AccountID{account},
		}
		cancel := s.traceSource.SubscribeToTraces(ctx, func(eventData []byte) {
			s.sendEvent(event{
				Name:   events.TraceEvent,
				Method: "trace",
				Params: eventData,
			})
		}, options)
		s.traceSubscriptions[account] = cancel
		counter += 1
	}
	return fmt.Sprintf("success! %v new subscriptions created", counter)
}

func (s *session) unsubscribeFromTraces(params []string) string {
	var counter int
	for _, a := range params {
		account, err := tongo.ParseAddress(a)
		if err != nil {
			return fmt.Sprintf("failed to process '%v' account: %v", a, err)
		}
		if cancelFn, ok := s.traceSubscriptions[account.ID]; ok {
			cancelFn()
			delete(s.traceSubscriptions, account.ID)
			counter += 1
		}
	}
	return fmt.Sprintf("success! %v subscription(s) removed", counter)
}

func mempoolParamsToOptions(params []string) (*sources.SubscribeToMempoolOptions, error) {
	if len(params) == 0 {
		return &sources.SubscribeToMempoolOptions{}, nil
	}
	if len(params) > 1 {
		return nil, fmt.Errorf("failed to process params: supported only one parameter")
	}
	parts := strings.Split(params[0], "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("failed to process params: invalid format")
	}
	if strings.ToLower(parts[0]) != "accounts" {
		return nil, fmt.Errorf("failed to process params: invalid format")
	}
	accounts := strings.Split(parts[1], ",")
	accountIDs := make([]tongo.AccountID, 0, len(accounts))
	for _, accountStr := range accounts {
		address, err := tongo.ParseAddress(accountStr)
		if err != nil {
			return nil, fmt.Errorf("failed to process params: %v", err)
		}
		accountIDs = append(accountIDs, address.ID)
	}
	return &sources.SubscribeToMempoolOptions{Accounts: accountIDs}, nil
}

func (s *session) subscribeToMempool(ctx context.Context, params []string) string {
	if s.mempoolSubscription != nil {
		return fmt.Sprintf("you are already subscribed to mempool")
	}
	options, err := mempoolParamsToOptions(params)
	if err != nil {
		return err.Error()
	}
	cancelFn, err := s.mempool.SubscribeToMessages(ctx, func(eventData []byte) {
		s.sendEvent(event{Method: "mempool_message", Params: eventData, Name: events.MempoolEvent})
	}, *options)
	if err != nil {
		return err.Error()
	}
	s.mempoolSubscription = cancelFn
	return fmt.Sprintf("success! you have subscribed to mempool")
}

func (s *session) unsubscribeFromMempool() string {
	if s.mempoolSubscription == nil {
		return fmt.Sprintf("you are not subscribed to mempool")
	}
	s.mempoolSubscription()
	s.mempoolSubscription = nil
	return fmt.Sprintf("success! you have unsubscribed from mempool")
}

func jsonRPCResponseMessage(message string, id uint64, jsonrpc, method string) (JsonRPCResponse, error) {
	mes, err := json.Marshal(message)
	if err != nil {
		return JsonRPCResponse{}, err
	}
	resp := JsonRPCResponse{
		ID:      id,
		JSONRPC: jsonrpc,
		Method:  method,
		Result:  mes,
	}
	return resp, nil
}

func (s *session) writeResponse(message string, request JsonRPCRequest) error {
	resp, err := jsonRPCResponseMessage(message, request.ID, request.JSONRPC, request.Method)
	if err != nil {
		return err
	}
	return s.conn.WriteJSON(resp)
}
