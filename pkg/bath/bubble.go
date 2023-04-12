package bath

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/core"
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
	return fmt.Sprintf("success: %v bounce: %v, bounced: %v,  account: %v, body: %v", b.success, b.bounce, b.bounced, b.account, b.decodedBody)
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

	accountWasActiveAtComputingTime bool
}

func (a Account) Is(i abi.ContractInterface) bool {
	return slices.Contains(a.Interfaces, i)
}

func (b BubbleTx) ToAction() *Action {
	if b.external {
		return nil
	}
	if b.opCode != nil && *b.opCode != 0 && b.accountWasActiveAtComputingTime && !b.account.Is(abi.Wallet) {
		operation := fmt.Sprintf("0x%x", *b.opCode)
		payload := ""
		if b.decodedBody != nil {
			operation = b.decodedBody.Operation
			payload = strings.TrimLeft(dumpCallArgs(b.decodedBody.Value, ""), "\n")
		}
		return &Action{
			SmartContractExec: &SmartContractAction{
				TonAttached: b.inputAmount,
				Executor:    b.inputFrom.Address, //can't be null because we check IsExternal
				Contract:    b.account.Address,
				Operation:   operation,
				Payload:     payload,
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
		s, ok := b.decodedBody.Value.(abi.TextCommentMsgBody)
		if ok {
			converted := string(s.Text)
			a.TonTransfer.Comment = &converted
		}
	}
	return a
}

func dumpCallArgs(v any, ident string) string {
	t := reflect.TypeOf(v)
	switch t.Kind() {
	case reflect.Struct:
		val := reflect.ValueOf(v)
		s := ""
		for i := 0; i < val.NumField(); i++ {
			s += fmt.Sprintf("\n%s%s:%s", ident, t.Field(i).Name, dumpCallArgs(val.Field(i).Interface(), ident+"  "))
		}
		return s
	default:
		return fmt.Sprintf("%s%+v", ident, v)
	}
}

func (b BubbleTx) operation(name string) bool {
	return b.decodedBody != nil && b.decodedBody.Operation == name
}
func FromTrace(trace *core.Trace) *Bubble {
	return fromTrace(trace, nil)
}

func fromTrace(trace *core.Trace, source *Account) *Bubble {
	btx := BubbleTx{
		success:                         trace.Success,
		account:                         Account{Address: trace.Account, Interfaces: trace.AccountInterfaces},
		external:                        trace.InMsg == nil || trace.InMsg.IsExternal(),
		accountWasActiveAtComputingTime: trace.Type != core.OrdinaryTx || trace.ComputePhase == nil || trace.ComputePhase.SkipReason != tlb.ComputeSkipReasonNoState,
	}
	accounts := []tongo.AccountID{trace.Account}
	if source != nil {
		accounts = append(accounts, source.Address)
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
		Accounts: accounts,
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
