package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
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
	eventCh             chan event
	subscriptions       map[tongo.AccountID]sources.CancelFn
	mempoolSubscription sources.CancelFn
	pingInterval        time.Duration
	subscriptionLimit   int
}

type event struct {
	Method string
	Params []byte
}

func newSession(logger *zap.Logger, txSource sources.TransactionSource, mempool sources.MemPoolSource, conn *websocket.Conn) *session {
	return &session{
		logger: logger,
		// TODO: use elastic channel to be sure transactionDispatcher doesn't hang
		eventCh:           make(chan event, 100),
		conn:              conn,
		mempool:           mempool,
		txSource:          txSource,
		subscriptions:     map[tongo.AccountID]sources.CancelFn{},
		pingInterval:      5 * time.Second,
		subscriptionLimit: subscriptionLimit,
	}
}

func (s *session) cancel() {
	for _, cancelFn := range s.subscriptions {
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
				err = s.conn.WriteJSON(response)
			case request := <-requestCh:
				var response string
				switch request.Method {
				case "subscribe_account":
					response = s.subscribeToTransactions(ctx, request.Params)
				case "subscribe_mempool":
					response = s.subscribeToMempool(ctx)
				case "unsubscribe_account":
					response = s.unsubscribe(request.Params)
				}
				err = s.writeResponse(response, request)
			case <-time.After(s.pingInterval):
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

func (s *session) subscribeToTransactions(ctx context.Context, params []string) string {
	accounts := make([]tongo.AccountID, 0, len(params))
	for _, a := range params {
		accountID, err := tongo.ParseAccountID(a)
		if err != nil {
			return fmt.Sprintf("failed to process '%v' account: %v", a, err)
		}
		accounts = append(accounts, accountID)
	}
	if len(s.subscriptions)+len(accounts) > s.subscriptionLimit {
		return fmt.Sprintf("you have reached the limit of %v subscriptions", s.subscriptionLimit)
	}
	var counter int
	for _, account := range accounts {
		if _, ok := s.subscriptions[account]; ok {
			continue
		}
		options := sources.SubscribeToTransactionsOptions{
			Accounts: []tongo.AccountID{account},
		}
		cancel := s.txSource.SubscribeToTransactions(ctx, func(eventData []byte) {
			s.eventCh <- event{
				Method: "account_transaction",
				Params: eventData,
			}
		}, options)
		s.subscriptions[account] = cancel
		counter += 1
	}
	return fmt.Sprintf("success! %v new subscriptions created", counter)
}

func (s *session) unsubscribe(params []string) string {
	return "not supported yet"
}

func (s *session) subscribeToMempool(ctx context.Context) string {
	if s.mempoolSubscription != nil {
		return fmt.Sprintf("you are already subscribed to mempool")
	}
	cancelFn, err := s.mempool.SubscribeToMessages(ctx, func(eventData []byte) {
		s.eventCh <- event{Method: "mempool_message", Params: eventData}
	})
	if err != nil {
		return err.Error()
	}
	s.mempoolSubscription = cancelFn
	return fmt.Sprintf("success! you have subscribed to mempool")
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
