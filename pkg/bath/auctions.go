package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type AuctionBidBubble struct {
	Type           string
	Amount         int64
	Nft            *core.NftItem
	NftAddress     *tongo.AccountID
	Bidder         tongo.AccountID
	Auction        tongo.AccountID
	PreviousBidder *tongo.AccountID //maybe don't requered
	Success        bool
}

type AuctionBidAction struct {
	Type       string
	Amount     int64
	Nft        *core.NftItem
	NftAddress *tongo.AccountID
	Bidder     tongo.AccountID
	Auction    tongo.AccountID
}

func (a AuctionBidBubble) ToAction() *Action {
	return &Action{
		AuctionBid: &AuctionBidAction{
			Type:       a.Type,
			Amount:     a.Amount,
			Nft:        a.Nft,
			NftAddress: a.NftAddress,
			Bidder:     a.Bidder,
			Auction:    a.Auction,
		},
		Type:    AuctionBid,
		Success: a.Success,
	}
}

func FindAuctionBidFragmentSimple(bubble *Bubble) bool {
	txBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !txBubble.account.Is(abi.Telemint) {
		return false
	}
	if txBubble.opCode != nil {
		return false
	}
	if txBubble.inputFrom == nil {
		return false
	}

	newBubble := Bubble{
		Info: AuctionBidBubble{
			Type:       "tg",
			Amount:     txBubble.inputAmount,
			Auction:    txBubble.account.Address,
			NftAddress: &txBubble.account.Address,
			Bidder:     txBubble.inputFrom.Address,
			Success:    txBubble.success,
		},
		Accounts:  bubble.Accounts,
		Children:  bubble.Children,
		ValueFlow: bubble.ValueFlow,
	}
	*bubble = newBubble
	return true
}

var TgAuctionV1InitialBidStraw = Straw[AuctionBidBubble]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.TelemintDeployMsgOp)},
	Builder: func(newAction *AuctionBidBubble, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Type = "tg"
		newAction.Amount = tx.inputAmount
		newAction.Auction = tx.account.Address
		newAction.Bidder = tx.inputFrom.Address
		return nil
	},
	SingleChild: &Straw[AuctionBidBubble]{
		CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0x299a3e15)},
		Builder: func(newAction *AuctionBidBubble, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = tx.success
			newAction.NftAddress = &tx.account.Address
			return nil
		},
	},
}
