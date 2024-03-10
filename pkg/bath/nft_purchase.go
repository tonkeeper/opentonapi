package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type BubbleNftPurchase struct {
	Success     bool
	Buyer       tongo.AccountID
	Seller      tongo.AccountID
	Nft         tongo.AccountID
	AuctionType NftAuctionType
	Price       int64
}

func (b BubbleNftPurchase) ToAction() *Action {
	return &Action{
		NftPurchase: &NftPurchaseAction{
			Nft:         b.Nft,
			Buyer:       b.Buyer,
			Seller:      b.Seller,
			AuctionType: b.AuctionType,
			Price:       b.Price,
		},
		Success: b.Success,
		Type:    NftPurchase,
	}
}

func auctionType(account *Account) NftAuctionType { //todo: switch to new gg interfaces
	if account.Is(abi.NftSaleV2) {
		return GetGemsAuction
	}
	return BasicAuction
}

var NftPurchaseStraw = Straw[BubbleNftPurchase]{
	CheckFuncs: []bubbleCheck{
		IsTx,
		HasInterface(abi.NftSale),
		HasEmptyBody,             //all buy transactions has empty body
		AmountInterval(1, 1<<62), //externals has zero value
		func(bubble *Bubble) bool {
			tx := bubble.Info.(BubbleTx)
			return tx.additionalInfo != nil && tx.additionalInfo.NftSaleContract != nil && tx.additionalInfo.NftSaleContract.Owner != nil
		}},
	Builder: func(newAction *BubbleNftPurchase, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		sale := tx.additionalInfo.NftSaleContract //safe to use, because we checked it in CheckFuncs
		newAction.Success = tx.success
		newAction.Seller = *sale.Owner
		newAction.AuctionType = auctionType(&tx.account)
		newAction.Buyer = tx.inputFrom.Address //safe because we checked not external in CheckFuncs
		newAction.Price = sale.NftPrice
		return nil
	},
	Children: []Straw[BubbleNftPurchase]{
		{
			CheckFuncs: []bubbleCheck{IsNftTransfer},
			Builder: func(newAction *BubbleNftPurchase, bubble *Bubble) error {
				newAction.Nft = bubble.Info.(BubbleNftTransfer).account.Address
				return nil
			},
		},
	},
}
