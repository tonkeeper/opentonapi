package bath

import (
	"github.com/arnac-io/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"
)

type InscriptionMintAction struct {
	Minter ton.AccountID
	Amount uint64
	Ticker string
	Type   string
}

type InscriptionTransferAction struct {
	Src, Dst ton.AccountID
	Amount   uint64
	Ticker   string
	Type     string
}

func ConvertToInscriptionActions(msgs []core.InscriptionMessage) map[ton.Bits256][]Action {
	actions := make(map[ton.Bits256][]Action)
	for _, msg := range msgs {
		var action Action
		switch msg.Operation {
		case "transfer":
			action.Type = InscriptionTransfer
			action.InscriptionTransfer = &InscriptionTransferAction{
				Src:    msg.Source,
				Dst:    *msg.Dest,
				Ticker: msg.Ticker,
				Amount: msg.Amount,
				Type:   "ton20",
			}
		case "mint":
			action.Type = InscriptionMint
			action.InscriptionMint = &InscriptionMintAction{
				Minter: msg.Source,
				Ticker: msg.Ticker,
				Type:   "ton20",
				Amount: msg.Amount,
			}
		default:
			continue
		}
		action.Success = msg.Success
		actions[msg.Hash] = append(actions[msg.Hash], action)
	}
	return actions
}
