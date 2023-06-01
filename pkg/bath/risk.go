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

func (r Risk) AddTon(amount int64) Risk {
	return Risk{
		Ton:     r.Ton + amount,
		Jettons: r.Jettons,
		Nfts:    r.Nfts,
	}
}

func (r Risk) AddJettons(jettonMaster tongo.AccountID, jettonWallet tongo.AccountID, amount big.Int) Risk {
	jettons := r.Jettons
	if _, ok := jettons[jettonMaster]; !ok {
		jettons[jettonMaster] = &JettonQuantity{
			WalletAddress: jettonWallet,
		}
	}
	jettons[jettonMaster].Quantity = *(&big.Int{}).Add(&amount, &jettons[jettonMaster].Quantity)
	return Risk{
		Ton:     r.Ton,
		Jettons: jettons,
		Nfts:    r.Nfts,
	}
}

func (r Risk) AddNFT(nft tongo.AccountID) Risk {
	return Risk{
		Ton:     r.Ton,
		Jettons: r.Jettons,
		Nfts:    append(r.Nfts, nft),
	}
}
