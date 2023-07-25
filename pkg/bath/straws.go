package bath

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/wallet"
)

// Straw extracts information from the given bubble and its children and modifies the bubble if needed.
// If the bubble is modified this function return true.
type Straw func(bubble *Bubble) (success bool)

var DefaultStraws = []Straw{
	FindNFTTransfer,
	FindJettonTransfer,
	FindAuctionBidFragmentSimple,
	FindInitialSubscription,
	FindExtendedSubscription,
	FindUnSubscription,
	FindNftPurchase,
	FindDepositStake,
	FindRecoverStake,
	FindTFNominatorAction,
	FindSTONfiSwap,
	// FindContractDeploy goes after other straws
	// because it inserts ContractDeploy actions in an action chain.
	// Usually, a straw looks for a strict sequence of bubbles like (Jetton Transfer) -> (Jetton Notify) and
	// such a straw would be broken with (Jetton Transfer) -> (Contract Deploy) -> (Jetton Notify).
	FindContractDeploy,
}

func FindNFTTransfer(bubble *Bubble) bool {
	nftBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !nftBubble.operation(abi.NftTransferMsgOp) {
		return false
	}
	transfer := nftBubble.decodedBody.Value.(abi.NftTransferMsgBody)
	newBubble := Bubble{
		Info: BubbleNftTransfer{
			success:   nftBubble.success,
			account:   nftBubble.account,
			sender:    nftBubble.inputFrom,
			recipient: parseAccount(transfer.NewOwner),
			payload:   cellToTextComment(boc.Cell(transfer.ForwardPayload.Value)),
		},
		Accounts:            append(bubble.Accounts, nftBubble.account.Address),
		ValueFlow:           bubble.ValueFlow,
		ContractDeployments: bubble.ContractDeployments,
	}
	newBubble.Children = ProcessChildren(bubble.Children,
		func(child *Bubble) *Merge {
			tx, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !tx.operation(abi.ExcessMsgOp) {
				return nil
			}
			newBubble.ValueFlow.Merge(child.ValueFlow)
			newBubble.MergeContractDeployments(child)
			newBubble.Accounts = append(newBubble.Accounts, child.Accounts...)
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
			newBubble.Accounts = append(newBubble.Accounts, child.Accounts...)
			newBubble.ValueFlow.Merge(child.ValueFlow)
			newBubble.MergeContractDeployments(child)
			return &Merge{children: child.Children}
		})
	*bubble = newBubble
	return true
}

type BubbleNftTransfer struct {
	success   bool
	account   Account
	sender    *Account
	recipient *Account
	payload   any //todo: replace any
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
	switch c := b.payload.(type) {
	case string:
		a.NftItemTransfer.Comment = &c
	case EncryptedComment:
		a.NftItemTransfer.EncryptedComment = &c
	}
	return &a
}

func cellToTextComment(payloadCell boc.Cell) any {
	var payload wallet.TextComment
	if err := tlb.Unmarshal(&payloadCell, &payload); err == nil {
		return string(payload)
	}
	payloadCell.ResetCounters()
	op, err := payloadCell.ReadUint(32)
	if uint32(op) == abi.EncryptedTextCommentMsgOpCode {
		var b tlb.Bytes
		if tlb.Unmarshal(&payloadCell, &b) == nil {
			return EncryptedComment{CipherText: b, EncryptionType: "simple"}
		}
	}
	if err == nil {
		return fmt.Sprintf("Call: 0x%x", op)
	}
	return nil
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
		payload: cellToTextComment(boc.Cell(intention.ForwardPayload.Value)),
	}
	if transferBubbleInfo.additionalInfo != nil && transferBubbleInfo.additionalInfo.JettonMaster != nil {
		transfer.master = *transferBubbleInfo.additionalInfo.JettonMaster
	}
	newBubble := Bubble{
		Children:            bubble.Children,
		ValueFlow:           bubble.ValueFlow,
		Accounts:            bubble.Accounts,
		ContractDeployments: bubble.ContractDeployments,
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
				newBubble.ValueFlow.Merge(notify.ValueFlow)
				newBubble.MergeContractDeployments(notify)
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
				newBubble.MergeContractDeployments(child)
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
						newBubble.MergeContractDeployments(excess)
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
						newBubble.MergeContractDeployments(notify)
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

type BubbleJettonTransfer struct {
	sender, recipient             *Account
	senderWallet, recipientWallet tongo.AccountID
	master                        tongo.AccountID
	amount                        tlb.VarUInteger16
	success                       bool
	payload                       any
}

func (b BubbleJettonTransfer) ToAction() (action *Action) {
	a := Action{
		JettonTransfer: &JettonTransferAction{
			Jetton:           b.master,
			Recipient:        b.recipient.Addr(),
			Sender:           b.sender.Addr(),
			RecipientsWallet: b.recipientWallet,
			SendersWallet:    b.senderWallet,
			Amount:           b.amount,
		},
		Success: b.success,
		Type:    JettonTransfer,
	}
	switch c := b.payload.(type) {
	case string:
		a.JettonTransfer.Comment = &c
	case EncryptedComment:
		a.JettonTransfer.EncryptedComment = &c
	}
	return &a
}
