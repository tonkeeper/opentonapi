package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

// BubbleSTONfiSwap contains information about a swap operation at the STONfi dex.
type BubbleSTONfiSwap struct {
	AmountIn        uint64
	AmountOut       uint64
	UserWallet      tongo.AccountID
	STONfiRouter    tongo.AccountID
	JettonWalletIn  tongo.AccountID
	JettonMasterIn  tongo.AccountID
	JettonWalletOut tongo.AccountID
	JettonMasterOut tongo.AccountID
	Success         bool
}

func (b BubbleSTONfiSwap) ToAction() *Action {
	return &Action{
		STONfiSwap: &STONfiSwapAction{
			UserWallet:      b.UserWallet,
			STONfiRouter:    b.STONfiRouter,
			JettonWalletIn:  b.JettonWalletIn,
			JettonMasterIn:  b.JettonMasterIn,
			JettonWalletOut: b.JettonWalletOut,
			JettonMasterOut: b.JettonMasterOut,
			AmountIn:        b.AmountIn,
			AmountOut:       b.AmountOut,
		},
		Type:    STONfiSwap,
		Success: b.Success,
	}
}

func FindSTONfiSwap(bubble *Bubble) bool {
	jettonTx, ok := bubble.Info.(BubbleJettonTransfer)
	if !ok {
		return false
	}
	if len(bubble.Children) != 1 {
		return false
	}
	child := bubble.Children[0]
	swapTx, ok := child.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !swapTx.account.Is(abi.StonfiPool) {
		return false
	}
	if !swapTx.operation(abi.StonfiSwapMsgOp) {
		return false
	}
	if swapTx.inputFrom == nil {
		return false
	}
	pool := swapTx.additionalInfo.STONfiPool
	if pool == nil {
		return false
	}
	swap := swapTx.decodedBody.Value.(abi.StonfiSwapMsgBody)
	sender, err := tongo.AccountIDFromTlb(swap.SenderAddress)
	if err != nil || sender == nil {
		return false
	}
	jettonWalletIn := *sender
	jettonWalletOut := pool.Token1
	toAddress, err := tongo.AccountIDFromTlb(swap.ToAddress)
	if err != nil || toAddress == nil {
		return false
	}
	userWallet, err := tongo.AccountIDFromTlb(swap.Addrs.Value.FromUser)
	if err != nil || userWallet == nil {
		return false
	}
	token0Out := false
	if *sender == pool.Token1 {
		token0Out = true
		jettonWalletOut = pool.Token0
	}
	stonfiSwap := BubbleSTONfiSwap{
		UserWallet:      *userWallet,
		JettonWalletIn:  jettonWalletIn,
		JettonWalletOut: jettonWalletOut,
		JettonMasterIn:  swapTx.additionalInfo.JettonMasters[jettonWalletIn],
		JettonMasterOut: swapTx.additionalInfo.JettonMasters[jettonWalletOut],
		AmountIn:        uint64(swap.JettonAmount),
		Success:         false,
	}
	newBubble := Bubble{
		Accounts:  append(child.Accounts, bubble.Accounts...),
		ValueFlow: child.ValueFlow,
	}
	newBubble.ValueFlow.Merge(bubble.ValueFlow)
	newBubble.Children = ProcessChildren(child.Children,
		func(payment *Bubble) *Merge {
			tx, ok := payment.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !tx.operation(abi.StonfiPaymentRequestMsgOp) {
				return nil
			}
			paymentRequest := tx.decodedBody.Value.(abi.StonfiPaymentRequestMsgBody)
			owner, err := tongo.AccountIDFromTlb(paymentRequest.Owner)
			if err != nil || owner == nil {
				return nil
			}
			if *owner == *toAddress {
				stonfiSwap.Success = true
				if token0Out {
					stonfiSwap.AmountOut = uint64(paymentRequest.Params.Value.Amount0Out)
				} else {
					stonfiSwap.AmountOut = uint64(paymentRequest.Params.Value.Amount1Out)
				}
			}
			stonfiSwap.STONfiRouter = tx.account.Address
			newBubble.ValueFlow.Merge(payment.ValueFlow)
			newBubble.Accounts = append(newBubble.Accounts, payment.Accounts...)
			children := ProcessChildren(payment.Children,
				func(jettonTransfer *Bubble) *Merge {
					tx, ok := jettonTransfer.Info.(BubbleJettonTransfer)
					if !ok {
						return nil
					}
					if tx.recipient.Address != jettonTx.sender.Address {
						return nil
					}
					newBubble.ValueFlow.Merge(jettonTransfer.ValueFlow)
					return &Merge{children: jettonTransfer.Children}
				})
			return &Merge{children: children}
		})
	newBubble.Info = stonfiSwap
	*bubble = newBubble
	return true
}
