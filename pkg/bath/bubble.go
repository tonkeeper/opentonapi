package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"golang.org/x/exp/maps"
)

type Bubble struct {
	Info     actioner
	Accounts []tongo.AccountID
	Children []*Bubble
	Fee      Fee
}

type Fee struct {
	WhoPay  tongo.AccountID
	Compute int64
	Storage int64
	Deposit int64
}

type Straw func(bubble *Bubble) (success bool)

type actioner interface {
	ToAction() *Action
}

type BubbleTx struct {
	success     bool
	inputAmount int64
	inputFrom   *tongo.AccountID
	bounce      bool
	bounced     bool
	external    bool
	account     tongo.AccountID
	opCode      *uint32
	decodedBody *core.DecodedMessageBody
}

func (b BubbleTx) ToAction() *Action {
	if b.external {
		return nil
	}
	return &Action{
		TonTransfer: &TonTransferAction{
			Amount:    b.inputAmount,
			Recipient: b.account,
			Sender:    *b.inputFrom, //can't be null because we check IsExternal
			Refund:    nil,
		},

		Success: true,
		Type:    TonTransfer,
	}
}

func FromTrace(trace *core.Trace) *Bubble {
	btx := BubbleTx{
		success:  trace.Success,
		account:  trace.Account,
		external: trace.InMsg == nil || trace.InMsg.IsExtenal(),
	}
	if trace.InMsg != nil {
		btx.bounce = trace.InMsg.Bounce
		btx.bounced = trace.InMsg.Bounced
		btx.inputAmount = trace.InMsg.Value
		btx.opCode = trace.InMsg.OpCode
		btx.decodedBody = trace.InMsg.DecodedBody
		btx.inputFrom = trace.InMsg.Source
	}

	b := Bubble{
		Info:     btx,
		Accounts: []tongo.AccountID{trace.Account},
		Children: make([]*Bubble, len(trace.Children)),
		Fee: Fee{
			WhoPay:  trace.Account,
			Compute: trace.OtherFee,
			Storage: trace.StorageFee,
		},
	}

	for i, c := range trace.Children {
		b.Children[i] = FromTrace(c)
	}

	return &b
}

func CollectActions(bubble *Bubble) ([]Action, []Fee) {
	fees := make(map[tongo.AccountID]Fee)
	actions := collectActions(bubble, fees)
	return actions, maps.Values(fees)
}

func collectActions(bubble *Bubble, fees map[tongo.AccountID]Fee) []Action {
	var actions []Action
	a := bubble.Info.ToAction()
	if a != nil {
		actions = append(actions, *a)
	}
	for _, c := range bubble.Children {
		a := collectActions(c, fees)
		actions = append(actions, a...)
		f := fees[bubble.Fee.WhoPay]
		f.WhoPay = bubble.Fee.WhoPay
		f.Storage += bubble.Fee.Storage
		f.Deposit += bubble.Fee.Deposit
		f.Compute += bubble.Fee.Compute
		fees[bubble.Fee.WhoPay] = f
	}
	return actions
}

var DefaultStraws = []Straw{
	FindNFTTransfer,
}

func FindNFTTransfer(bubble *Bubble) bool {
	return false
}
