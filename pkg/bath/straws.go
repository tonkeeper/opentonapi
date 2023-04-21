package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

// Straw extracts information from the given bubble and its children and modifies the bubble if needed.
// If the bubble is modified this function return true.
type Straw func(bubble *Bubble) (success bool)

var DefaultStraws = []Straw{
	FindNFTTransfer,
	FindJettonTransfer,
}

func parseAccount(a tlb.MsgAddress) *Account {
	o, err := tongo.AccountIDFromTlb(a)
	if err == nil && o != nil {
		return &Account{Address: *o}
	}
	return nil
}

type Merge struct {
	children []*Bubble
}

func ProcessChildren(children []*Bubble, fns ...func(child *Bubble) *Merge) []*Bubble {
	var newChildren []*Bubble
	for _, child := range children {
		merged := false
		for _, fn := range fns {
			merge := fn(child)
			if merge != nil {
				newChildren = append(newChildren, merge.children...)
				merged = true
				break
			}
		}
		if !merged {
			newChildren = append(newChildren, child)
		}
	}
	return newChildren
}

func FindNFTTransfer(bubble *Bubble) bool {
	nftBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !nftBubble.operation("NftTransfer") {
		return false
	}
	transfer := nftBubble.decodedBody.Value.(abi.NftTransferMsgBody)
	newBubble := Bubble{
		Info: BubbleNftTransfer{
			success:   nftBubble.success,
			account:   nftBubble.account,
			sender:    nftBubble.inputFrom,
			recipient: parseAccount(transfer.NewOwner),
		},
		Accounts: append(bubble.Accounts, nftBubble.account.Address),
		Fee:      bubble.Fee,
	}
	newBubble.Fee.WhoPay = nftBubble.inputFrom.Address
	newBubble.Fee.Deposit += nftBubble.inputAmount
	newBubble.Children = ProcessChildren(bubble.Children,
		func(child *Bubble) *Merge {
			tx, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !tx.operation("Excess") {
				return nil
			}
			newBubble.Fee.Add(child.Fee)
			newBubble.Fee.Refund += tx.inputAmount
			newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
			return &Merge{children: child.Children}
		},
		func(child *Bubble) *Merge {
			tx, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !tx.operation("NftOwnershipAssigned") {
				return nil
			}
			newBubble.Fee.Add(child.Fee)
			newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
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
	if !jettonBubble.operation("JettonTransfer") {
		return false
	}
	intention := jettonBubble.decodedBody.Value.(abi.JettonTransferMsgBody)
	recipient, err := tongo.AccountIDFromTlb(intention.Destination)
	if err != nil || recipient == nil {
		return false
	}

	transfer := BubbleJettonTransfer{
		sender:       jettonBubble.inputFrom,
		senderWallet: jettonBubble.account.Address,
		master:       tongo.AccountID{},
		amount:       intention.Amount,
		recipient: &Account{
			Address: *recipient,
		},
		payload: nil, //todo: do
	}
	if master, ok := jettonBubble.additionalInfo["jetton_master"]; ok {
		transfer.master, _ = master.(tongo.AccountID)
	}
	newBubble := Bubble{
		Accounts: append(bubble.Accounts, jettonBubble.account.Address),
		Children: bubble.Children,
		Fee:      bubble.Fee,
	}
	if jettonBubble.success {
		newBubble.Children = ProcessChildren(bubble.Children,
			func(child *Bubble) *Merge {
				tx, ok := child.Info.(BubbleTx)
				if !ok {
					return nil
				}
				if !tx.operation("JettonInternalTransfer") {
					return nil
				}
				if tx.success {
					transfer.success = true
				}
				transfer.recipientWallet = tx.account.Address
				children := ProcessChildren(child.Children,
					func(excess *Bubble) *Merge {
						tx, ok := child.Info.(BubbleTx)
						if !ok {
							return nil
						}
						if !tx.operation("Excess") {
							return nil
						}
						return &Merge{children: excess.Children}
					},
					func(notify *Bubble) *Merge {
						tx, ok := child.Info.(BubbleTx)
						if !ok {
							return nil
						}
						if !tx.operation("JettonNotify") {
							return nil
						}
						transfer.success = true
						if transfer.recipient.Address != tx.account.Address {
							transfer.success = false
						}
						transfer.recipient.Interfaces = tx.account.Interfaces
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
