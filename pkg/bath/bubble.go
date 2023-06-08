package bath

import (
	"fmt"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
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
	ToAction(metaResolver) *Action
}

func FromTrace(trace *core.Trace) *Bubble {
	return fromTrace(trace)
}

func fromTrace(trace *core.Trace) *Bubble {
	btx := BubbleTx{
		success:                         trace.Success,
		transactionType:                 trace.Transaction.Type,
		account:                         Account{Address: trace.Account, Interfaces: trace.AccountInterfaces},
		external:                        trace.InMsg == nil || trace.InMsg.IsExternal(),
		accountWasActiveAtComputingTime: trace.Type != core.OrdinaryTx || trace.ComputePhase == nil || trace.ComputePhase.SkipReason != tlb.ComputeSkipReasonNoState,
		additionalInfo:                  trace.AdditionalInfo,
	}
	accounts := []tongo.AccountID{trace.Account}
	var source *Account
	if trace.InMsg != nil && trace.InMsg.Source != nil {
		source = &Account{
			Address: *trace.InMsg.Source,
		}
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
	aggregatedFee := trace.TotalFee
	b := Bubble{
		Info:     btx,
		Accounts: accounts,
		Children: make([]*Bubble, len(trace.Children)),
		ValueFlow: &ValueFlow{
			Accounts: map[tongo.AccountID]*AccountValueFlow{
				trace.Account: {
					Ton: inputAmount,
				},
			},
		},
	}
	for _, outMsg := range trace.OutMsgs {
		b.ValueFlow.AddTons(trace.Account, -outMsg.Value)
		aggregatedFee += outMsg.FwdFee
	}
	for i, c := range trace.Children {
		if c.InMsg != nil {
			// If an outbound message has a corresponding InMsg,
			// we have removed it from OutMsgs to avoid duplicates.
			// That's why we add tons here
			b.ValueFlow.AddTons(trace.Account, -c.InMsg.Value)
			aggregatedFee += c.InMsg.FwdFee
		}
		b.Children[i] = fromTrace(c)
	}
	b.ValueFlow.Accounts[trace.Account].Fees = aggregatedFee
	return &b
}
