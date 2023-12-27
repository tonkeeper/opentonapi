package bath

import (
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
	DNSRenewStraw.Merge,
	FindTFNominatorAction,
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
	if transferBubbleInfo.additionalInfo != nil {
		if master, ok := transferBubbleInfo.additionalInfo.JettonMaster(transferBubbleInfo.account.Address); ok {
			transfer.master = master
		}
	}
	newBubble := Bubble{
		Children:  bubble.Children,
		ValueFlow: bubble.ValueFlow,
		Accounts:  bubble.Accounts,
	}
	// TODO: check they have the same master

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
				transfer.amount = notifyTx.decodedBody.Value.(abi.JettonNotifyMsgBody).Amount
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
	if !transfer.isWrappedTon {
		newBubble.ValueFlow.AddJettons(*recipient, transfer.master, big.Int(intention.Amount))
		newBubble.ValueFlow.SubJettons(transferBubbleInfo.inputFrom.Address, transfer.master, big.Int(intention.Amount))
	}
	newBubble.Info = transfer
	*bubble = newBubble
	return true
}
