package bath

import (
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
