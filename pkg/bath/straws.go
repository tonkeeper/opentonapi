package bath

import (
	"fmt"
	"github.com/tonkeeper/opentonapi/internal/g"
	"math/big"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

// StrawFunc extracts information from the given bubble and its children and modifies the bubble if needed.
// If the bubble is modified this function return true.
type StrawFunc func(bubble *Bubble) (success bool)

var JettonTransfersBurnsMints = []StrawFunc{
	FindJettonTransfer,
	JettonBurnStraw.Merge,
	DedustLPJettonMintStraw.Merge,
	WtonMintStraw.Merge,
}

var NFTStraws = []StrawFunc{
	NftTransferStraw.Merge,
	NftTransferNotifyStraw.Merge,
}

var DefaultStraws = []StrawFunc{
	NftTransferStraw.Merge,
	NftTransferNotifyStraw.Merge,
	FindJettonTransfer,
	JettonBurnStraw.Merge,
	WtonMintStraw.Merge,
	FindNftPurchase,
	StonfiSwapStraw.Merge,
	DedustSwapStraw.Merge,
	TgAuctionV1InitialBidStraw.Merge,
	FindAuctionBidFragmentSimple,
	DedustLPJettonMintStraw.Merge,
	MegatonFiJettonSwap.Merge,
	FindInitialSubscription,
	FindExtendedSubscription,
	FindUnSubscription,
	DepositLiquidStakeStraw.Merge,
	PendingWithdrawRequestLiquidStraw.Merge,
	ElectionsDepositStakeStraw.Merge,
	ElectionsRecoverStakeStraw.Merge,
	DepositTFStakeStraw.Merge,
	WithdrawTFStakeRequestStraw.Merge,
	WithdrawStakeImmediatelyStraw.Merge,
	WithdrawLiquidStake.Merge,
	FindTFNominatorAction,
}

var NftTransferNotifyStraw = Straw[BubbleNftTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftOwnershipAssignedMsgOp)},
	Builder: func(newAction *BubbleNftTransfer, bubble *Bubble) error {
		receiverTx := bubble.Info.(BubbleTx)
		transfer := receiverTx.decodedBody.Value.(abi.NftOwnershipAssignedMsgBody)
		newAction.success = true
		if receiverTx.inputFrom == nil {
			return fmt.Errorf("nft transfer notify without sender")
		}
		newAction.account = *receiverTx.inputFrom
		if newAction.sender == nil {
			newAction.sender = parseAccount(transfer.PrevOwner)
		}
		newAction.recipient = &receiverTx.account
		newAction.payload = transfer.ForwardPayload.Value
		return nil
	},
}

var NftTransferStraw = Straw[BubbleNftTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftTransferMsgOp)},
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
		Optional(NftTransferNotifyStraw),
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

func FindJettonTransfer(bubble *Bubble) bool {
	transferBubbleInfo, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !transferBubbleInfo.operation(abi.JettonTransferMsgOp) {
		return false
	}
	intention := transferBubbleInfo.decodedBody.Value.(abi.JettonTransferMsgBody)
	recipient, err := tongo.AccountIDFromTlb(intention.Destination)
	if err != nil || recipient == nil {
		return false
	}

	transfer := BubbleJettonTransfer{
		sender:       transferBubbleInfo.inputFrom,
		senderWallet: transferBubbleInfo.account.Address,
		master:       tongo.AccountID{},
		amount:       intention.Amount,
		recipient: &Account{
			Address: *recipient,
		},
		payload: intention.ForwardPayload.Value,
	}
	if transferBubbleInfo.additionalInfo != nil && transferBubbleInfo.additionalInfo.JettonMaster != nil {
		transfer.master = *transferBubbleInfo.additionalInfo.JettonMaster
	}
	newBubble := Bubble{
		Children:  bubble.Children,
		ValueFlow: bubble.ValueFlow,
		Accounts:  bubble.Accounts,
	}
	newBubble.ValueFlow.AddJettons(*recipient, transfer.master, big.Int(intention.Amount))
	if transferBubbleInfo.success {
		newBubble.Children = ProcessChildren(bubble.Children,
			func(notify *Bubble) *Merge {
				// pTON sends a jetton-notify msg just after a jetton-transfer operation.
				notifyTx, ok := notify.Info.(BubbleTx)
				if !ok {
					return nil
				}
				if !notifyTx.operation(abi.JettonNotifyMsgOp) {
					return nil
				}
				transfer.success = true
				transfer.isWrappedTon = true
				newBubble.ValueFlow.Merge(notify.ValueFlow)
				newBubble.Accounts = append(newBubble.Accounts, notify.Accounts...)
				return &Merge{children: notify.Children}
			},
			func(child *Bubble) *Merge {
				receiveBubbleInfo, ok := child.Info.(BubbleTx)
				if !ok {
					return nil
				}
				if !receiveBubbleInfo.operation(abi.JettonInternalTransferMsgOp) {
					return nil
				}
				if receiveBubbleInfo.success {
					transfer.success = true
				}
				transfer.recipientWallet = receiveBubbleInfo.account.Address
				newBubble.Accounts = append(newBubble.Accounts, child.Accounts...)
				children := ProcessChildren(child.Children,
					func(excess *Bubble) *Merge {
						tx, ok := excess.Info.(BubbleTx)
						if !ok {
							return nil
						}
						if !tx.operation(abi.ExcessMsgOp) {
							return nil
						}
						newBubble.ValueFlow.Merge(excess.ValueFlow)
						newBubble.Accounts = append(newBubble.Accounts, excess.Accounts...)
						return &Merge{children: excess.Children}
					},
					func(notify *Bubble) *Merge {
						tx, ok := notify.Info.(BubbleTx)
						if !ok {
							return nil
						}
						if !tx.operation(abi.JettonNotifyMsgOp) {
							return nil
						}
						transfer.success = true
						if transfer.recipient.Address != tx.account.Address {
							transfer.success = false
						}
						transfer.recipient.Interfaces = tx.account.Interfaces
						newBubble.ValueFlow.Merge(notify.ValueFlow)
						newBubble.Accounts = append(newBubble.Accounts, notify.Accounts...)
						return &Merge{children: notify.Children}
					},
				)
				return &Merge{children: children}
			})
		if transfer.recipient == nil {
			transfer.recipient = parseAccount(intention.Destination)
		}
	}
	newBubble.Info = transfer
	*bubble = newBubble
	return true
}
