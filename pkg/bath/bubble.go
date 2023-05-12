package bath

import (
	"fmt"
	"strings"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

// Bubble represents a transaction in the beginning.
// But we can merge neighbour bubbles together
// if we find a known action pattern like an NFT Transfer or a SmartContractExecution in a trace.
type Bubble struct {
	Info      actioner
	Accounts  []tongo.AccountID
	Children  []*Bubble
	ValueFlow *ValueFlow
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

func (a Account) Is(i abi.ContractInterface) bool {
	return slices.Contains(a.Interfaces, i)
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
		additionalInfo:                  trace.AdditionalInfo,
	}
	accounts := []tongo.AccountID{trace.Account}
	if source != nil {
		accounts = append(accounts, source.Address)
	}
	var inputAmount int64
	if msg := trace.InMsg; msg != nil {
		btx.bounce = msg.Bounce
		btx.bounced = msg.Bounced
		btx.inputAmount = msg.Value
		inputAmount = msg.Value
		btx.opCode = msg.OpCode
		btx.decodedBody = msg.DecodedBody
		btx.inputFrom = source
		btx.init = msg.Init
	}
	b := Bubble{
		Info:     btx,
		Accounts: accounts,
		Children: make([]*Bubble, len(trace.Children)),
		ValueFlow: &ValueFlow{
			Accounts: map[tongo.AccountID]*AccountValueFlow{
				trace.Account: {
					Ton:  inputAmount,
					Fees: trace.OtherFee + trace.StorageFee,
				},
			},
		},
	}
	for _, outMsg := range trace.OutMsgs {
		b.ValueFlow.AddTons(trace.Account, -outMsg.Value)
	}
	for i, c := range trace.Children {
		b.Children[i] = fromTrace(c, &btx.account)
	}
	return &b
}
