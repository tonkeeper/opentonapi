package bath

import (
	"fmt"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo/abi"
)

var NftTransferNotifyStraw = Straw[BubbleNftTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.NftItem)},
	Children: []Straw[BubbleNftTransfer]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftOwnershipAssignedMsgOp)},
			Builder: func(newAction *BubbleNftTransfer, bubble *Bubble) error {
				receiverTx := bubble.Info.(BubbleTx)
				transfer := receiverTx.decodedBody.Value.(abi.NftOwnershipAssignedMsgBody)
				newAction.success = true
				if receiverTx.inputFrom == nil {
					return fmt.Errorf("nft transfer notify without sender")
				}
				newAction.account = *receiverTx.inputFrom
				newAction.recipient = &receiverTx.account
				newAction.payload = transfer.ForwardPayload.Value
				return nil
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
			Optional:   true,
		},
	},
}

var NftTransferStraw = Straw[BubbleNftTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftTransferMsgOp), HasInterface(abi.NftItem)},
	Builder: func(newAction *BubbleNftTransfer, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		transfer := tx.decodedBody.Value.(abi.NftTransferMsgBody)
		newAction.account = tx.account
		newAction.success = tx.success
		newAction.sender = tx.inputFrom
		newAction.payload = transfer.ForwardPayload.Value
		if newAction.recipient == nil {
			newAction.recipient = parseAccount(transfer.NewOwner)
		}
		return nil
	},
	Children: []Straw[BubbleNftTransfer]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftOwnershipAssignedMsgOp)},
			Optional:   true,
			Builder: func(newAction *BubbleNftTransfer, bubble *Bubble) error {
				receiverTx := bubble.Info.(BubbleTx)
				transfer := receiverTx.decodedBody.Value.(abi.NftOwnershipAssignedMsgBody)
				newAction.success = true
				newAction.payload = transfer.ForwardPayload.Value
				return nil
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
			Optional:   true,
		},
	},
}

type BubbleNftTransfer struct {
	success   bool
	account   Account
	sender    *Account
	recipient *Account
	payload   abi.NFTPayload
}

func (b BubbleNftTransfer) ToAction() (action *Action) {
	a := Action{
		NftItemTransfer: &NftTransferAction{
			Recipient: b.recipient.Addr(),
			Sender:    b.sender.Addr(),
			Nft:       b.account.Address,
		},
		Success: b.success,
		Type:    NftItemTransfer,
	}
	switch b.payload.SumType {
	case abi.TextCommentNFTOp:
		a.NftItemTransfer.Comment = g.Pointer(string(b.payload.Value.(abi.TextCommentNFTPayload).Text))
	case abi.EncryptedTextCommentNFTOp:
		a.NftItemTransfer.EncryptedComment = &EncryptedComment{
			CipherText:     b.payload.Value.(abi.EncryptedTextCommentNFTPayload).CipherText,
			EncryptionType: "simple",
		}
	case abi.EmptyNFTOp:
	default:
		if b.payload.SumType != abi.UnknownNFTOp {
			a.NftItemTransfer.Comment = g.Pointer("Call: " + b.payload.SumType)
		} else if b.recipient != nil && b.recipient.Is(abi.Wallet) {
			// we don't want to show the scary "Call: Ugly HEX" to the wallet contract
		} else if b.payload.OpCode != nil {
			a.NftItemTransfer.Comment = g.Pointer(fmt.Sprintf("Call: 0x%08x", *b.payload.OpCode))
		}
	}
	return &a
}

//type BubbleNftMint struct {
//	Minter *ton.AccountID
//	Owner *ton.AccountID
//	Item ton.AccountID
//}
//
//func (b BubbleNftMint) ToAction() *Action {
//	//TODO implement me
//	panic("implement me")
//}
//
//var NFTMintStraw = Straw[BubbleNftMint]{
//
//	Children: []Straw[BubbleNftMint]{
//		{
//
//		},
//	},
//	CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
//}
