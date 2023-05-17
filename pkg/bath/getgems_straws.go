package bath

import (
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type BubbleGetGemsNftPurchase struct {
	Success  bool
	NewOwner tongo.AccountID
	Nft      tongo.AccountID
}

func (b BubbleGetGemsNftPurchase) ToAction(book addressBook) *Action {
	return &Action{
		GetGemsNftPurchase: &GetGemsNftPurchaseAction{
			Nft:      b.Nft,
			NewOwner: b.NewOwner,
		},
		Success: b.Success,
		Type:    GetGemsNftPurchase,
		SimplePreview: SimplePreview{
			Value: g.Pointer[int64](1),
			Accounts: []tongo.AccountID{
				b.NewOwner, b.Nft,
			},
			MessageID: nftPurchaseMessageID,
		},
	}
}

func FindGetGemsNftPurchase(bubble *Bubble) bool {
	txBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !txBubble.account.Is(abi.NftSaleGetgems) {
		return false
	}
	if len(bubble.Children) != 4 && len(bubble.Children[3].Children) != 2 {
		return false
	}
	if !noOpCode(bubble.Children[0]) {
		return false
	}
	if !noOpCode(bubble.Children[1]) {
		return false
	}
	if !noOpCode(bubble.Children[2]) {
		return false
	}
	if !operation(bubble.Children[3], abi.NftTransferMsgOp) {
		return false
	}
	if !operation(bubble.Children[3].Children[0], abi.NftOwnershipAssignedMsgOp) {
		return false
	}
	if !operation(bubble.Children[3].Children[1], abi.ExcessMsgOp) {
		return false
	}
	newBubble := Bubble{
		Accounts:  bubble.Accounts,
		ValueFlow: bubble.ValueFlow,
	}
	var newOwner tongo.AccountID
	var nft tongo.AccountID
	var success bool
	newBubble.Children = ProcessChildren(bubble.Children,
		func(child *Bubble) *Merge {
			tx, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !tx.operation(abi.NftTransferMsgOp) {
				return nil
			}
			nft = tx.account.Address
			success = tx.success
			newBubble.ValueFlow.Merge(child.ValueFlow)
			newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
			children := ProcessChildren(child.Children,
				func(child *Bubble) *Merge {
					tx, ok := child.Info.(BubbleTx)
					if !ok {
						return nil
					}
					if !tx.operation(abi.ExcessMsgOp) {
						return nil
					}
					newBubble.ValueFlow.Merge(child.ValueFlow)
					newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
					return &Merge{children: child.Children}
				},
				func(child *Bubble) *Merge {
					tx, ok := child.Info.(BubbleTx)
					if !ok {
						return nil
					}
					if !tx.operation(abi.NftOwnershipAssignedMsgOp) {
						return nil
					}
					newOwner = tx.account.Address
					newBubble.ValueFlow.AddNFT(newOwner, nft, 1)
					newBubble.ValueFlow.Merge(child.ValueFlow)
					newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
					return &Merge{children: child.Children}
				},
			)
			return &Merge{children: children}
		},
		func(child *Bubble) *Merge {
			tx, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if tx.opCode != nil {
				return nil
			}
			newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
			newBubble.ValueFlow.Merge(child.ValueFlow)
			return &Merge{children: child.Children}
		})
	newBubble.Info = BubbleGetGemsNftPurchase{
		Success:  success,
		Nft:      nft,
		NewOwner: newOwner,
	}
	*bubble = newBubble
	return true
}
