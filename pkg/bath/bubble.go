package bath

import (
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"golang.org/x/exp/slices"
	"strings"
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
	Refund  int64
}

func (f *Fee) Add(f2 Fee) {
	f.Compute += f2.Compute
	f.Storage += f2.Storage
	f.Deposit += f2.Deposit
	f.Refund += f2.Refund
}

func (f Fee) Total() int64 {
	return f.Deposit + f.Compute + f.Storage - f.Refund
}

func (b BubbleTx) String() string {
	return fmt.Sprintf("buccess: %v bounce: %v, bounced: %v,  account: %v, body: %v", b.success, b.bounce, b.bounced, b.account, b.decodedBody)
}

func (b Bubble) String() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "%T: ", b.Info)
	prefix := "    "
	fmt.Fprintf(&buf, " %+v\n", b.Info)
	for _, c := range b.Children {
		for _, l := range strings.Split(c.String(), "\n") {
			if l == "" {
				continue
			}
			buf.WriteString(prefix + l + "\n")
		}
	}
	return buf.String()
}

func TakeBubble(bubbles []*Bubble, check func(bubble *Bubble) bool) (sl []*Bubble, b *Bubble) {
	for i := range bubbles {
		if check(bubbles[i]) {
			b, bubbles[i] = bubbles[i], bubbles[len(bubbles)-1]
			return bubbles[:len(bubbles)-1], b
		}
	}
	return bubbles, nil
}

type actioner interface {
	ToAction() *Action
}

type Account struct {
	Address    tongo.AccountID
	Interfaces []abi.ContractInterface
}

func (a *Account) Addr() *tongo.AccountID {
	if a == nil {
		return nil
	}
	return &a.Address
}

type BubbleTx struct {
	success     bool
	inputAmount int64
	inputFrom   *Account
	bounce      bool
	bounced     bool
	external    bool
	account     Account
	opCode      *uint32
	decodedBody *core.DecodedMessageBody
	init        []byte
}

func (a Account) Is(i abi.ContractInterface) bool {
	return slices.Contains(a.Interfaces, i)
}

func (b BubbleTx) ToAction() *Action {
	if b.external {
		return nil
	}
	if b.opCode != nil && *b.opCode != 0 {
		operation := fmt.Sprintf("0x%x", *b.opCode)
		if b.decodedBody != nil {
			operation = b.decodedBody.Operation
		}
		return &Action{
			SmartContractExec: &SmartContractAction{
				TonAttached: b.inputAmount,
				Executor:    b.inputFrom.Address, //can't be null because we check IsExternal
				Contract:    b.account.Address,
				Operation:   operation,
				Payload:     "", //todo: add payload
			},
			Success: b.success,
			Type:    SmartContractExec,
		}
	}
	a := &Action{
		TonTransfer: &TonTransferAction{
			Amount:    b.inputAmount,
			Recipient: b.account.Address,
			Sender:    b.inputFrom.Address, //can't be null because we check IsExternal
			Refund:    nil,
		},

		Success: true,
		Type:    TonTransfer,
	}
	if b.decodedBody != nil {
		s := string(b.decodedBody.Value.(abi.TextCommentMsgBody).Text)
		a.TonTransfer.Comment = &s
	}
	return a
}

func FromTrace(trace *core.Trace) *Bubble {
	return fromTrace(trace, nil)
}

func fromTrace(trace *core.Trace, source *Account) *Bubble {
	btx := BubbleTx{
		success:  trace.Success,
		account:  Account{Address: trace.Account, Interfaces: trace.AccountInterfaces},
		external: trace.InMsg == nil || trace.InMsg.IsExternal(),
	}
	if trace.InMsg != nil {
		btx.bounce = trace.InMsg.Bounce
		btx.bounced = trace.InMsg.Bounced
		btx.inputAmount = trace.InMsg.Value
		btx.opCode = trace.InMsg.OpCode
		btx.decodedBody = trace.InMsg.DecodedBody
		btx.inputFrom = source
		btx.init = trace.InMsg.Init
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
		b.Children[i] = fromTrace(c, &btx.account)
	}

	return &b
}

func MergeAllBubbles(bubble *Bubble, straws []Straw) {
	for _, s := range straws {
		for {
			success := recursiveMerge(bubble, s)
			if success {
				continue
			}
			break
		}
	}
}

func recursiveMerge(bubble *Bubble, s Straw) bool {
	if s(bubble) {
		return true
	}
	for _, b := range bubble.Children {
		if recursiveMerge(b, s) {
			return true
		}
	}
	return false
}
