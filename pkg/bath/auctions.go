package bath

import (
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type AuctionBidBubble struct {
	Type           NftAuctionType
	Amount         int64
	Nft            *core.NftItem
	NftAddress     *tongo.AccountID
	Bidder         tongo.AccountID
	Auction        tongo.AccountID
	PreviousBidder *tongo.AccountID //maybe don't requered
	Username       string
	Success        bool
}

type AuctionBidAction struct {
	Type       NftAuctionType
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

var StrawFindAuctionBidFragmentSimple = Straw[AuctionBidBubble]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.Teleitem), HasEmptyBody, AmountInterval(1, 1<<62)},
	Builder: func(newAction *AuctionBidBubble, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Type = DnsTgAuction
		newAction.Amount = tx.inputAmount
		newAction.Bidder = tx.inputFrom.Address
		newAction.Success = tx.success
		newAction.Auction = tx.account.Address
		newAction.NftAddress = &tx.account.Address
		return nil
	},
}

var TgAuctionV1InitialBidStraw = Straw[AuctionBidBubble]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.TelemintDeployMsgOp)},
	Builder: func(newAction *AuctionBidBubble, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		body := tx.decodedBody.Value.(abi.TelemintDeployMsgBody)
		newAction.Type = DnsTgAuction
		newAction.Amount = tx.inputAmount
		newAction.Bidder = tx.inputFrom.Address
		newAction.Username = string(body.Msg.Username)
		return nil
	},
	SingleChild: &Straw[AuctionBidBubble]{
		CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0x299a3e15)},
		Builder: func(newAction *AuctionBidBubble, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = tx.success

			newAction.Auction = tx.account.Address
			newAction.NftAddress = &tx.account.Address
			if tx.additionalInfo.EmulatedTeleitemNFT != nil {
				newAction.Nft = &core.NftItem{
					Address:           tx.account.Address,
					Index:             tx.additionalInfo.EmulatedTeleitemNFT.Index,
					CollectionAddress: tx.additionalInfo.EmulatedTeleitemNFT.CollectionAddress,
					Verified:          tx.additionalInfo.EmulatedTeleitemNFT.Verified,
					Transferable:      false,
					Metadata: map[string]interface{}{
						"name":  newAction.Username,
						"image": fmt.Sprintf("https://nft.fragment.com/username/%v.webp", newAction.Username),
					},
				}
			}
			return nil
		},
	},
}

var StrawAuctionBigGetgems = Straw[AuctionBidBubble]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.NftAuctionV1), HasEmptyBody, AmountInterval(1, 1<<62)},
	Builder: func(newAction *AuctionBidBubble, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Auction = tx.account.Address
		newAction.Bidder = tx.inputFrom.Address
		newAction.Amount = tx.inputAmount
		newAction.Success = tx.success
		newAction.Type = GetGemsAuction
		if tx.additionalInfo != nil && tx.additionalInfo.NftSaleContract != nil {
			newAction.NftAddress = &tx.additionalInfo.NftSaleContract.Item
		}
		return nil
	},
}

var StrawAuctionBuyGetgems = Straw[BubbleNftPurchase]{
	CheckFuncs: []bubbleCheck{Is(AuctionBidBubble{})},
	Builder: func(newAction *BubbleNftPurchase, bubble *Bubble) error {
		bid := bubble.Info.(AuctionBidBubble)
		newAction.Buyer = bid.Bidder
		newAction.Nft = *bid.NftAddress
		newAction.Price = bid.Amount
		newAction.AuctionType = bid.Type
		newAction.Success = bid.Success
		return nil
	},
	SingleChild: &Straw[BubbleNftPurchase]{
		CheckFuncs: []bubbleCheck{IsNftTransfer},
	},
}

var StrawAuctionBuyFragments = Straw[BubbleNftPurchase]{
	CheckFuncs: []bubbleCheck{Is(AuctionBidBubble{})},
	Builder: func(newAction *BubbleNftPurchase, bubble *Bubble) error {
		bid := bubble.Info.(AuctionBidBubble)
		newAction.Buyer = bid.Bidder
		newAction.Nft = *bid.NftAddress
		newAction.Price = bid.Amount
		newAction.AuctionType = bid.Type
		newAction.Success = bid.Success
		return nil
	},
	SingleChild: &Straw[BubbleNftPurchase]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftOwnershipAssignedMsgOp)},
		Builder: func(newAction *BubbleNftPurchase, bubble *Bubble) error {
			newAction.Seller = parseAccount(bubble.Info.(BubbleTx).decodedBody.Value.(abi.NftOwnershipAssignedMsgBody).PrevOwner).Address
			return nil
		},
	},
}
