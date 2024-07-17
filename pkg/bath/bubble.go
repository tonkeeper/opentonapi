package bath

import (
	"fmt"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

// Bubble represents a transaction in the beginning.
// But we can merge neighbour bubbles together
// if we find a known action pattern like an NFT Transfer or a SmartContractExecution in a trace.
type Bubble struct {
	Info        actioner
	Accounts    []tongo.AccountID
	Children    []*Bubble
	ValueFlow   *ValueFlow
	Transaction []ton.Bits256
}

// ContractDeployment holds information about initialization of a contract.
type ContractDeployment struct {
	//// initInterfaces is a list of interfaces implemented by the code of stateInit.
	initInterfaces []abi.ContractInterface
	success        bool
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
		additionalInfo:                  trace.AdditionalInfo(),
	}

	accounts := []tongo.AccountID{trace.Account}
	var source *Account
	if trace.InMsg != nil && trace.InMsg.Source != nil {
		source = &Account{
			Address: *trace.InMsg.Source,
		}
		accounts = append(accounts, source.Address)
	}
	var initInterfaces []abi.ContractInterface
	if msg := trace.InMsg; msg != nil {
		btx.bounce = msg.Bounce
		btx.bounced = msg.Bounced
		btx.inputAmount += msg.Value
		btx.inputAmount += msg.IhrFee
		btx.opCode = msg.OpCode
		btx.decodedBody = msg.DecodedBody
		btx.inputFrom = source
		btx.init = msg.Init
		initInterfaces = msg.InitInterfaces
	}
	var inputAmount int64
	if trace.Transaction.CreditPhase != nil {
		inputAmount = int64(trace.Transaction.CreditPhase.CreditGrams)
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
		Transaction: []ton.Bits256{trace.Hash},
	}
	for _, outMsg := range trace.OutMsgs {
		b.ValueFlow.AddTons(trace.Account, -outMsg.Value)
	}
	for i, c := range trace.Children {
		if c.InMsg != nil {
			// If an outbound message has a corresponding InMsg,
			// we have removed it from OutMsgs to avoid duplicates.
			// That's why we add tons here
			b.ValueFlow.AddTons(trace.Account, -c.InMsg.Value)

			// We want to include ihr_fee into msg.Value
			aggregatedFee -= c.InMsg.IhrFee
			b.ValueFlow.Accounts[trace.Account].Ton -= c.InMsg.IhrFee
		}

		b.Children[i] = fromTrace(c)
	}
	if actionPhase := trace.Transaction.ActionPhase; actionPhase != nil {
		aggregatedFee += int64(actionPhase.FwdFees)
		aggregatedFee -= int64(actionPhase.TotalFees)
	}
	contractDeployed := trace.EndStatus == tlb.AccountActive && trace.OrigStatus != tlb.AccountActive
	if contractDeployed {
		b.Children = append(b.Children, &Bubble{
			Info: BubbleContractDeploy{
				Contract:              trace.Account,
				Success:               true,
				AccountInitInterfaces: initInterfaces,
			},
			Accounts:    []tongo.AccountID{trace.Account},
			ValueFlow:   &ValueFlow{},
			Transaction: []ton.Bits256{trace.Hash},
		})
	}
	b.ValueFlow.Accounts[trace.Account].Ton -= aggregatedFee
	b.ValueFlow.Accounts[trace.Account].Fees = aggregatedFee
	return &b
}
