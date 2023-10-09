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

func auctionType(account *Account) NftAuctionType {
	if account.Is(abi.NftSaleV2) {
		return GetGemsAuction
	}
	return BasicAuction
}

func getGemsOpCodeOK(opcode *uint32) bool {
	if opcode == nil {
		return true
	}
	// according to getgems source code
	// 0 and 2 are OK
	// https://github.com/getgems-io/nft-contracts/blob/debcd8516b91320fa9b23bff6636002d639e3f26/packages/contracts/sources/nft-fixprice-sale-v3.fc#L228-L249
	return *opcode == 2 || *opcode == 0
}

func FindNftPurchase(bubble *Bubble) bool {
	txBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !txBubble.account.Is(abi.NftSale) {
		return false
	}
	if txBubble.additionalInfo == nil || txBubble.additionalInfo.NftSaleContract == nil {
		return false
	}
	saleContract := txBubble.additionalInfo.NftSaleContract
	if saleContract.Owner == nil {
		// TODO:
		return false
	}
	transfers := 0
	for _, child := range bubble.Children {
		if transfer, ok := child.Info.(BubbleNftTransfer); ok {
			// we don't want to merge a successful ton transfer with a failed nft transfer.
			if !transfer.success {
				return false
			}
			transfers += 1
		}
	}
	if transfers != 1 {
		return false
	}
	if txBubble.account.Is(abi.NftSaleV2) {
		if !getGemsOpCodeOK(txBubble.opCode) {
			return false
		}
	}
	newBubble := Bubble{
		Accounts:  bubble.Accounts,
		ValueFlow: bubble.ValueFlow,
	}
	var nft tongo.AccountID
	var buyer tongo.AccountID
	newBubble.Children = ProcessChildren(bubble.Children,
		func(child *Bubble) *Merge {
			nftTransfer, ok := child.Info.(BubbleNftTransfer)
			if !ok {
				return nil
			}
			newBubble.ValueFlow.Merge(child.ValueFlow)
			nft = nftTransfer.account.Address
			newBubble.Accounts = append(newBubble.Accounts, nft)
			if nftTransfer.sender != nil {
				newBubble.Accounts = append(newBubble.Accounts, nftTransfer.sender.Address)
			}
			if nftTransfer.recipient != nil {
				buyer = nftTransfer.recipient.Address
				newBubble.Accounts = append(newBubble.Accounts, buyer)
			}
			return &Merge{children: child.Children}
		})
	newBubble.Info = BubbleNftPurchase{
		Success:     true,
		Nft:         nft,
		Buyer:       buyer,
		Seller:      *saleContract.Owner,
		AuctionType: auctionType(&txBubble.account),
		Price:       saleContract.NftPrice,
	}
	*bubble = newBubble
	return true
}
