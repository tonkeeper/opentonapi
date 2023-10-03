package main

import (
	"encoding/json"
	"fmt"

	"github.com/r3labs/sse/v2"
	"github.com/tonkeeper/tongo"
)

// MempoolEventData represents the data part of a new-pending-message event.
type MempoolEventData struct {
	BOC []byte `json:"boc"`
}

func subscribeToMempool() error {
	client := sse.NewClient("https://tonapi.io/v2/sse/mempool")
	// When working with tonapi.io, you should consider getting an API key at https://tonconsole.com/
	// because tonapi.io has per-ip limits for websocket.
	// client.Headers["Authorization"] = "bearer API-KEY"
	//
	// To work with a local version of opentonapi use http://127.0.0.1:8081/v2/sse/mempool
	//
	err := client.Subscribe("", func(msg *sse.Event) {
		switch string(msg.Event) {
		case "heartbeat":
			fmt.Printf("%v\n", string(msg.Event))
		case "message":
			data := MempoolEventData{}
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				fmt.Printf("json.Unmarshal() failed: %v, data: %#v", err, msg)
				return
			}
			fmt.Printf("new message's boc: %x\n", string(data.BOC))
		}
	})
	return err
}

// TransactionEventData represents the data part of a new-transaction event.
type TransactionEventData struct {
	AccountID tongo.AccountID `json:"account_id"`
	Lt        uint64          `json:"lt"`
	TxHash    string          `json:"tx_hash"`
}

func subscribeToTransactions() error {
	client := sse.NewClient("https://tonapi.io/v2/sse/accounts/transactions?accounts=-1:5555555555555555555555555555555555555555555555555555555555555555")
	// When working with tonapi.io, you should consider getting an API key at https://tonconsole.com/
	// because tonapi.io has per-ip limits for websocket.
	// client.Headers["Authorization"] = "bearer API-KEY"
	//
	// To work with a local version of opentonapi use http://127.0.0.1:8081/v2/sse/accounts/transactions
	//
	err := client.Subscribe("", func(msg *sse.Event) {
		switch string(msg.Event) {
		case "heartbeat":
			fmt.Printf("%v\n", string(msg.Event))
		case "message":
			data := TransactionEventData{}
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				fmt.Printf("json.Unmarshal() failed: %v, data: %#v", err, msg)
				return
			}
			fmt.Printf("accountID: %v, lt: %v, tx hash: %x\n", data.AccountID.ToRaw(), data.Lt, data.TxHash)
		}
	})
	return err
}

// TraceEventData represents a notification about a completed trace.
type TraceEventData struct {
	AccountIDs []tongo.AccountID `json:"accounts"`
	Hash       string            `json:"hash"`
}

func subscribeToTraces() error {
	client := sse.NewClient("https://tonapi.io/v2/sse/accounts/traces?accounts=-1:5555555555555555555555555555555555555555555555555555555555555555")
	// When working with tonapi.io, you should consider getting an API key at https://tonconsole.com/
	// because tonapi.io has per-ip limits for websocket.
	// client.Headers["Authorization"] = "bearer API-KEY"
	//
	// To work with a local version of opentonapi use http://127.0.0.1:8081/v2/sse/accounts/transactions
	//
	err := client.Subscribe("", func(msg *sse.Event) {
		switch string(msg.Event) {
		case "heartbeat":
			fmt.Printf("%v\n", string(msg.Event))
		case "message":
			data := TraceEventData{}
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				fmt.Printf("json.Unmarshal() failed: %v, data: %#v", err, msg)
				return
			}
			var accounts []string
			for _, account := range data.AccountIDs {
				accounts = append(accounts, account.ToRaw())
			}
			fmt.Printf("trace hash %v, accounts: %v\n", data.Hash, accounts)
		}
	})
	return err
}

func main() {
	if err := subscribeToTraces(); err != nil {
		panic(err)
	}
	if err := subscribeToMempool(); err != nil {
		panic(err)
	}
	if err := subscribeToTransactions(); err != nil {
		panic(err)
	}
	select {}
}
