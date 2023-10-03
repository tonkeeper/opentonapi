package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

// JsonRPCRequest represents a request in the JSON-RPC protocol supported by "/v2/websocket" endpoint.
type JsonRPCRequest struct {
	ID      uint64   `json:"id,omitempty"`
	JSONRPC string   `json:"jsonrpc,omitempty"`
	Method  string   `json:"method,omitempty"`
	Params  []string `json:"params,omitempty"`
}

// JsonRPCResponse represents a response in the JSON-RPC protocol supported by "/v2/websocket" endpoint.
type JsonRPCResponse struct {
	ID      uint64          `json:"id,omitempty"`
	JSONRPC string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func main() {

	header := http.Header{}
	//
	// When working with tonapi.io, you should consider getting an API key at https://tonconsole.com/
	// because tonapi.io has per-ip limits for websocket.
	// header.Set("Authorization", "bearer API-KEY")
	//
	// To work with a local version of opentonapi use ws://127.0.0.1:8081/v2/websocket
	conn, _, err := websocket.DefaultDialer.Dial("wss://tonapi.io/v2/websocket", header)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				panic(err)
			}
			var response JsonRPCResponse
			if err := json.Unmarshal(msg, &response); err != nil {
				panic(err)
			}
			fmt.Printf("method: %v, params: %v, result: %v\n", response.Method, string(response.Params), string(response.Result))
		}
	}()
	// subscribe to notifications about transactions
	request := JsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "subscribe_account",
		Params:  []string{"-1:5555555555555555555555555555555555555555555555555555555555555555"},
	}
	if err = conn.WriteJSON(request); err != nil {
		panic(err)
	}
	// use the same connection to subscribe to notifications about pending messages
	request = JsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "subscribe_mempool",
	}
	if err = conn.WriteJSON(request); err != nil {
		panic(err)
	}
	// subscribe to notifications about traces
	request = JsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "subscribe_trace",
		Params:  []string{"-1:5555555555555555555555555555555555555555555555555555555555555555"},
	}
	if err = conn.WriteJSON(request); err != nil {
		panic(err)
	}
	// do nothing
	select {}
}
