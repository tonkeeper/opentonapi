package bath

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
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
	FindInitialSubscription,
	FindExtendedSubscription,
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
		Accounts:  append(bubble.Accounts, nftBubble.account.Address),
		ValueFlow: bubble.ValueFlow,
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

func (b BubbleNftTransfer) ToAction(book addressBook) (action *Action) {
	a := Action{
		NftItemTransfer: &NftTransferAction{
			Recipient: b.recipient.Addr(),
			Sender:    b.sender.Addr(),
			Nft:       b.account.Address,
		},
		Success: b.success,
		Type:    NftItemTransfer,
		SimplePreview: SimplePreview{
			Name:      "NFT Transfer",
			MessageID: nftTransferMessageID,
			Accounts:  distinctAccounts(b.account.Address, b.sender.Address, b.recipient.Address),
			Value:     "1 NFT",
		},
	}
	if c, ok := b.payload.(string); ok {
		a.NftItemTransfer.Comment = &c
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
	if master, ok := transferBubbleInfo.additionalInfo["jetton_master"]; ok {
		transfer.master, _ = master.(tongo.AccountID)
	}
	newBubble := Bubble{
		Children:  bubble.Children,
		ValueFlow: bubble.ValueFlow,
		Accounts:  bubble.Accounts,
	}
	newBubble.ValueFlow.AddJettons(*recipient, transfer.master, big.Int(intention.Amount))
	if transferBubbleInfo.success {
		newBubble.Children = ProcessChildren(bubble.Children,
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

type BubbleJettonTransfer struct {
	sender, recipient             *Account
	senderWallet, recipientWallet tongo.AccountID
	master                        tongo.AccountID
	amount                        tlb.VarUInteger16
	success                       bool
	payload                       any
}

func (b BubbleJettonTransfer) ToAction(book addressBook) (action *Action) {
	jettonName := core.UnknownJettonName
	if book != nil {
		if jetton, ok := book.GetJettonInfoByAddress(b.master); ok {
			jettonName = jetton.Name
		}
	}
	amount := big.Int(b.amount)
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
		SimplePreview: SimplePreview{
			Name:      "Jetton Transfer",
			MessageID: jettonTransferMessageID,
			TemplateData: map[string]interface{}{
				"Value":      amount.Int64(),
				"JettonName": jettonName,
			},
			Accounts: distinctAccounts(b.recipient.Address, b.sender.Address, b.master),
			Value:    fmt.Sprintf("%v %v", amount.String(), jettonName),
		},
	}
	if c, ok := b.payload.(string); ok {
		a.JettonTransfer.Comment = &c
	}
	return &a
}
