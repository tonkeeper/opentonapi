package websocket

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
)

var (
	upgrader websocket.Upgrader // use default options
)

type JsonRPCRequest struct {
	ID      uint64   `json:"id,omitempty"`
	JSONRPC string   `json:"jsonrpc,omitempty"`
	Method  string   `json:"method,omitempty"`
	Params  []string `json:"params,omitempty"`
}

type JsonRPCResponse struct {
	ID      uint64          `json:"id,omitempty"`
	JSONRPC string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func Handler(logger *zap.Logger, txSource sources.TransactionSource) func(http.ResponseWriter, *http.Request, int) error {
	return func(w http.ResponseWriter, r *http.Request, connectionType int) error {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("failed to upgrade HTTP connection to websocket protocol",
				zap.Error(err),
				zap.String("remoteAddr", conn.RemoteAddr().String()))
			return err
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		session := newSession(logger, txSource, conn)
		requestCh := session.Run(ctx)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					return nil
				}
				return err
			}
			var request JsonRPCRequest
			if err = json.Unmarshal(msg, &request); err != nil {
				logger.Error("request unmarshalling error", zap.Error(err))
				return err
			}
			requestCh <- request
		}
	}
}
