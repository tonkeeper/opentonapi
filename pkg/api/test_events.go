package api

import (
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

//

const (
	testEventAccount     = "0:2cf3b5b8c891e517c9addbda1c0386a09ccacbb0e3faf630b51cfc8152325acb"
	testRecipientAccount = "0:e9a07c65998cd537d6ac2c4c9ddd73a299295527101328c87358508ccbf868fa"
	testEventID          = "a96d84940781cc29d3fb890384d35ba49cdd9d891a123a9f90939ddb57b09fc2"
)

func getTestActions() []oas.Action {
	return []oas.Action{
		{
			Type:   oas.ActionTypeTonTransfer,
			Status: oas.ActionStatusOk,
			TonTransfer: oas.OptTonTransferAction{
				Set: true,
				Value: oas.TonTransferAction{
					Sender: oas.AccountAddress{
						Address: testEventAccount,
						IsScam:  false,
					},
					Recipient: oas.AccountAddress{
						Address: testRecipientAccount,
						IsScam:  false,
					},
					Amount: 10_000_000,
				},
			},
			SimplePreview: oas.ActionSimplePreview{
				Name:        "Ton Transfer",
				Description: "Transferring 10_000_000 TON",
				Value: oas.OptString{
					Set:   true,
					Value: "10_000_000 TON",
				},
				Accounts: []oas.AccountAddress{
					{
						Address: testEventAccount,
						IsScam:  false,
					},
					{
						Address: testRecipientAccount,
						IsScam:  false,
					},
				},
			},
		},
	}
}

func getTestAccountEvent() oas.AccountEvent {
	return oas.AccountEvent{
		EventID: testEventID,
		Account: oas.AccountAddress{
			Address: testEventAccount,
			IsScam:  false,
		},
		Timestamp:  time.Now().Unix(),
		IsScam:     false,
		Lt:         int64(39228825000001),
		InProgress: true,
		Extra:      -5825767,
		Actions:    getTestActions(),
	}
}

func getTestEvent() oas.Event {
	now := time.Now().Unix()
	event := oas.Event{
		EventID:   testEventID,
		Timestamp: now,
		Actions:   getTestActions(),
		ValueFlow: []oas.ValueFlow{
			{
				Account: oas.AccountAddress{
					Address: testEventAccount,
					IsScam:  false,
				},
				Ton:  -10_000_000,
				Fees: 1,
			},
			{
				Account: oas.AccountAddress{
					Address: testRecipientAccount,
					IsScam:  false,
				},
				Ton:  10_000_000,
				Fees: 1,
			},
		},
		IsScam:     false,
		Lt:         int64(39226975000001),
		InProgress: true,
	}
	return event
}

func getTestTrace() oas.Trace {
	trace := oas.Trace{
		Transaction: getTestTransaction(),
		Interfaces:  []string{"wallet_v4r2", "wallet_v4", "wallet"},
	}
	return trace
}

func getTestTransaction() oas.Transaction {
	now := time.Now().Unix()

	tx := oas.Transaction{
		Hash: testEventID,
		Lt:   int64(39226975000001),
		Account: oas.AccountAddress{
			Address: testEventAccount,
			IsScam:  false,
		},
		Success:         false,
		Utime:           now,
		OrigStatus:      oas.AccountStatusActive,
		EndStatus:       oas.AccountStatusActive,
		TotalFees:       5266796,
		TransactionType: oas.TransactionTypeTransOrd,
		Aborted:         false,
		Destroyed:       false,
		InMsg: oas.OptMessage{
			Set: true,
			Value: oas.Message{
				CreatedLt: 39228825000001,
				Destination: oas.OptAccountAddress{
					Set: true,
					Value: oas.AccountAddress{
						Address: testEventAccount,
						IsScam:  false,
					},
				},
				CreatedAt: now,
				RawBody:   oas.OptString{Set: true, Value: "b5ee9c720101...0000000000"},
			},
		},
		OutMsgs: []oas.Message{
			{
				CreatedLt:   39228825000002,
				IhrDisabled: true,
				Value:       10_000_000,
				FwdFee:      1,
				Destination: oas.OptAccountAddress{
					Set: true,
					Value: oas.AccountAddress{
						Address: testRecipientAccount,
						IsScam:  false,
					},
				},
				Source: oas.OptAccountAddress{
					Set: true,
					Value: oas.AccountAddress{
						Address: testEventAccount,
						IsScam:  false,
					},
				},
				CreatedAt: now,
			},
		},
	}

	return tx
}
