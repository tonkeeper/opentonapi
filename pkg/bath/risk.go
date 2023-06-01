package bath

import (
	"math/big"

	"github.com/tonkeeper/tongo"
)

// Risk specifies assets that could be lost
// if a message would be sent to a malicious smart contract.
// It makes sense to understand the risk BEFORE sending a message to the blockchain.
type Risk struct {
	Ton     int64
	Jettons map[tongo.AccountID]*JettonQuantity
	Nfts    []tongo.AccountID
}

type JettonQuantity struct {
	WalletAddress tongo.AccountID
	Quantity      big.Int
}
