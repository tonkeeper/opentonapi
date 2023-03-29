package bath

import (
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

type Straw func(bubble *Bubble) (success bool)

var DefaultStraws = []Straw{
	FindNFTTransfer,
	FindJettonTransfer,
}

func FindNFTTransfer(bubble *Bubble) bool {
	nftBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if nftBubble.decodedBody == nil {
		return false
	}
	transfer, ok := nftBubble.decodedBody.Value.(abi.NftTransferMsgBody)
	if !ok {
		return false
	}
	var newOwner *Account
	if o, err := tongo.AccountIDFromTlb(transfer.NewOwner); err == nil && o != nil {
		newOwner = &Account{
			Address: *o,
		}
	}
	newBubble := Bubble{
		Info: BubbleNftTransfer{
			success:   nftBubble.success,
			account:   nftBubble.account,
			sender:    nftBubble.inputFrom,
			recipient: newOwner,
		},
		Accounts: append(bubble.Accounts, nftBubble.account.Address),
		Children: bubble.Children,
		Fee:      bubble.Fee,
	}
	newBubble.Fee.WhoPay = nftBubble.inputFrom.Address
	newBubble.Fee.Deposit += nftBubble.inputAmount
	var excessBubble, notifyBubble *Bubble
	newBubble.Children, excessBubble = TakeBubble(newBubble.Children, func(b *Bubble) bool {
		tx, ok := b.Info.(BubbleTx)
		return ok && tx.decodedBody != nil && tx.decodedBody.Operation == "Excess"
	})
	newBubble.Children, notifyBubble = TakeBubble(newBubble.Children, func(b *Bubble) bool {
		tx, ok := b.Info.(BubbleTx)
		return ok && tx.decodedBody != nil && tx.decodedBody.Operation == "NftOwnershipAssigned"
	})
	if excessBubble != nil {
		newBubble.Fee.Add(excessBubble.Fee)
		tx := excessBubble.Info.(BubbleTx)
		newBubble.Fee.Refund += tx.inputAmount
		newBubble.Children = append(newBubble.Children, excessBubble.Children...)
		newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
	}
	if notifyBubble != nil {
		newBubble.Fee.Add(notifyBubble.Fee)
		tx := notifyBubble.Info.(BubbleTx)
		newBubble.Children = append(newBubble.Children, notifyBubble.Children...)
		newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
	}
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
	return &Action{
		NftItemTransfer: &NftTransferAction{
			Comment:   nil, //todo: add
			Recipient: b.account.Addr(),
			Sender:    b.sender.Addr(),
			Nft:       b.account.Address,
		},
		Success: b.success,
		Type:    NftItemTransfer,
	}
}

func FindJettonTransfer(bubble *Bubble) bool {
	jettonBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if jettonBubble.decodedBody == nil {
		return false
	}
	if jettonBubble.decodedBody.Operation != "JettonTransfer" {
		return false
	}
	intention := jettonBubble.decodedBody.Value.(abi.JettonTransferMsgBody)
	transfer := BubbleJettonTransfer{
		sender:       jettonBubble.inputFrom,
		senderWallet: jettonBubble.account.Address,
		master:       tongo.AccountID{},
		amount:       intention.Amount,
		payload:      nil, //todo: do
	}
	newBubble := Bubble{
		Accounts: append(bubble.Accounts, jettonBubble.account.Address),
		Children: bubble.Children,
		Fee:      bubble.Fee,
	}
	var recipient *Account
	var receiveBubble, excessBubble, notifyBubble *Bubble
	if jettonBubble.success {
		newBubble.Children, receiveBubble = TakeBubble(newBubble.Children, func(b *Bubble) bool {
			tx, ok := b.Info.(BubbleTx)
			return ok && tx.decodedBody != nil && tx.decodedBody.Operation == "JettonInternalTransfer"
		})
		if receiveBubble != nil {
			tx := receiveBubble.Info.(BubbleTx)
			transfer.recipientWallet = tx.account.Address

			receiveBubble.Children, excessBubble = TakeBubble(receiveBubble.Children, func(b *Bubble) bool {
				tx, ok := b.Info.(BubbleTx)
				return ok && tx.decodedBody != nil && tx.decodedBody.Operation == "Excess"
			})
			receiveBubble.Children, notifyBubble = TakeBubble(receiveBubble.Children, func(b *Bubble) bool {
				tx, ok := b.Info.(BubbleTx)
				return ok && tx.opCode != nil && *tx.opCode == 0x7362d09c //todo: replace with text
			})
			if excessBubble != nil {
				newBubble.Children = append(newBubble.Children, excessBubble.Children...)
			}
			if notifyBubble != nil {
				newBubble.Children = append(newBubble.Children, notifyBubble.Children...)
				recipient = g.Pointer(notifyBubble.Info.(BubbleTx).account)
			} else {
				a, err := tongo.AccountIDFromTlb(intention.Destination)
				if a != nil && err == nil {
					recipient = &Account{
						Address: *a,
					}
				}
			}
			newBubble.Children = append(newBubble.Children, receiveBubble.Children...)
		}
	}
	transfer.recipient = recipient
	newBubble.Info = transfer

	newBubble.Fee.WhoPay = jettonBubble.inputFrom.Address
	*bubble = newBubble
	return true
}

type BubbleJettonTransfer struct {
	sender, recipient             *Account
	senderWallet, recipientWallet tongo.AccountID
	master                        tongo.AccountID
	amount                        tlb.VarUInteger16
	success                       bool
	payload                       any //todo: do something
}

func (b BubbleJettonTransfer) ToAction() (action *Action) {
	return &Action{
		JettonTransfer: &JettonTransferAction{
			Comment:          nil,
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
}
