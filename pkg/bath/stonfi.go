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
	STONfiPool      tongo.AccountID
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
			STONfiPool:      b.STONfiPool,
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
	bubbleTx, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !bubbleTx.account.Is(abi.StonfiPool) {
		return false
	}
	if !bubbleTx.operation(abi.StonfiSwapMsgOp) {
		return false
	}
	if bubbleTx.inputFrom == nil {
		return false
	}
	pool := bubbleTx.additionalInfo.STONfiPool
	if pool == nil {
		return false
	}
	swap := bubbleTx.decodedBody.Value.(abi.StonfiSwapMsgBody)
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
		STONfiPool:      bubbleTx.account.Address,
		JettonWalletIn:  jettonWalletIn,
		JettonWalletOut: jettonWalletOut,
		JettonMasterIn:  bubbleTx.additionalInfo.JettonMasters[jettonWalletIn],
		JettonMasterOut: bubbleTx.additionalInfo.JettonMasters[jettonWalletOut],
		AmountIn:        uint64(swap.JettonAmount),
		Success:         false,
	}
	newBubble := Bubble{
		Accounts:  bubble.Accounts,
		ValueFlow: bubble.ValueFlow,
	}
	newBubble.Children = ProcessChildren(bubble.Children,
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
			return &Merge{children: payment.Children}
		})
	newBubble.Info = stonfiSwap
	*bubble = newBubble
	return true
}
