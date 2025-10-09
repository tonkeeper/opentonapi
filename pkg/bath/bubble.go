package bath

import (
	"fmt"
	"github.com/tonkeeper/tongo/boc"
	"sort"
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

func fromTrace(trace *core.Trace, parent *core.Trace) *Bubble {
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
		if parent != nil {
			source.Interfaces = parent.AccountInterfaces
		}
		accounts = append(accounts, source.Address)
	}
	var initInterfaces []abi.ContractInterface
	if msg := trace.InMsg; msg != nil {
		btx.bounce = msg.Bounce
		btx.bounced = msg.Bounced
		btx.inputAmount += msg.Value
		if !msg.IhrDisabled {
			btx.inputAmount += msg.IhrFee
		}
		btx.inputExtraAmount = msg.ValueExtra
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

	for _, m := range trace.OutMsgs {
		if m.MsgType == core.ExtOutMsg {
			btx.externalOut = append(btx.externalOut, m)
		}
	}

	b := Bubble{
		Info:     btx,
		Accounts: accounts,
		Children: make([]*Bubble, len(trace.Children)),
		ValueFlow: &ValueFlow{
			Accounts: map[tongo.AccountID]*AccountValueFlow{
				// TODO: add extra currency
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
	sort.Slice(trace.Children, func(i, j int) bool {
		if trace.Children[i].InMsg == nil || trace.Children[j].InMsg == nil {
			return false
		}
		return trace.Children[i].InMsg.CreatedLt < trace.Children[j].InMsg.CreatedLt
	})
	for i, c := range trace.Children {
		if c.InMsg != nil {
			// If an outbound message has a corresponding InMsg,
			// we have removed it from OutMsgs to avoid duplicates.
			// That's why we add tons here
			b.ValueFlow.AddTons(trace.Account, -c.InMsg.Value)

			// We want to include ihr_fee into msg.Value
			if !c.InMsg.IhrDisabled {
				aggregatedFee -= c.InMsg.IhrFee
				b.ValueFlow.Accounts[trace.Account].Ton -= c.InMsg.IhrFee
			}
		}

		b.Children[i] = fromTrace(c, trace)
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
	switch {
	case btx.account.Is(abi.WalletV5R1):
		appendExtendedActionsW5(&b, trace.Hash)
	case btx.account.Is(abi.WalletV4R1) || btx.account.Is(abi.WalletV4R2):
		appendExtendedActionsV4(&b, trace.Hash)
	}
	b.ValueFlow.Accounts[trace.Account].Ton -= aggregatedFee
	b.ValueFlow.Accounts[trace.Account].Fees = aggregatedFee
	return &b
}

func appendExtendedActionsW5(b *Bubble, txHash tongo.Bits256) {
	btx := b.Info.(BubbleTx)
	if btx.decodedBody == nil {
		return
	}
	var extendedActions *abi.W5ExtendedActions
	switch btx.decodedBody.Operation {
	case abi.WalletSignedExternalV5R1ExtInMsgOp:
		msg := btx.decodedBody.Value.(abi.WalletSignedExternalV5R1ExtInMsgBody)
		extendedActions = msg.Extended
	case abi.WalletSignedInternalV5R1MsgOp:
		msg := btx.decodedBody.Value.(abi.WalletSignedInternalV5R1MsgBody)
		extendedActions = msg.Extended
	case abi.WalletExtensionActionV5R1MsgOp:
		msg := btx.decodedBody.Value.(abi.WalletExtensionActionV5R1MsgBody)
		extendedActions = msg.Extended
	}
	if extendedActions != nil {
		for _, ea := range *extendedActions {
			switch ea.SumType {
			case "AddExtension":
				addr, err := tongo.AccountIDFromTlb(ea.AddExtension.Addr)
				if err != nil || addr == nil {
					continue
				}
				b.Children = append(b.Children, &Bubble{
					Info: BubbleAddExtension{
						Wallet:    btx.account.Address,
						Extension: *addr,
						Success:   btx.success, // TODO: or check exit codes
					},
					Accounts:    []tongo.AccountID{btx.account.Address, *addr},
					ValueFlow:   &ValueFlow{},
					Transaction: []ton.Bits256{txHash},
				})
			case "RemoveExtension":
				addr, err := tongo.AccountIDFromTlb(ea.RemoveExtension.Addr)
				if err != nil || addr == nil {
					continue
				}
				b.Children = append(b.Children, &Bubble{
					Info: BubbleRemoveExtension{
						Wallet:    btx.account.Address,
						Extension: *addr,
						Success:   btx.success, // TODO: or check exit codes
					},
					Accounts:    []tongo.AccountID{btx.account.Address, *addr},
					ValueFlow:   &ValueFlow{},
					Transaction: []ton.Bits256{txHash},
				})
			case "SetSignatureAllowed":
				b.Children = append(b.Children, &Bubble{
					Info: BubbleSetSignatureAllowed{
						Wallet:           btx.account.Address,
						SignatureAllowed: ea.SetSignatureAllowed.Allowed,
						Success:          btx.success, // TODO: or check exit codes
					},
					Accounts:    []tongo.AccountID{btx.account.Address},
					ValueFlow:   &ValueFlow{},
					Transaction: []ton.Bits256{txHash},
				})
			}
		}
	}
}

func appendExtendedActionsV4(b *Bubble, txHash tongo.Bits256) {
	btx := b.Info.(BubbleTx)
	if btx.decodedBody == nil {
		return
	}
	newChild := Bubble{
		ValueFlow:   &ValueFlow{},
		Transaction: []ton.Bits256{txHash},
	}
	if btx.decodedBody.Operation == abi.WalletPluginDestructMsgOp {
		newChild.Info = BubbleRemoveExtension{Wallet: btx.account.Address, Extension: btx.inputFrom.Address, Success: btx.success} // TODO: or check exit codes
		newChild.Accounts = []tongo.AccountID{btx.account.Address, btx.inputFrom.Address}
		b.Children = append(b.Children, &newChild)
		return
	}
	if btx.decodedBody.Operation != abi.WalletSignedV4ExtInMsgOp {
		return
	}
	msg := btx.decodedBody.Value.(abi.WalletSignedV4ExtInMsgBody)
	var addr tongo.AccountID
	switch msg.Payload.SumType {
	case "DeployAndInstallPlugin":
		addr.Workchain = int32(msg.Payload.DeployAndInstallPlugin.PluginWorkchain)
		stateCell := boc.NewCell()
		err := tlb.Marshal(stateCell, msg.Payload.DeployAndInstallPlugin.StateInit)
		if err != nil {
			return
		}
		h, err := stateCell.Hash()
		if err != nil {
			return
		}
		copy(addr.Address[:], h[:])
		newChild.Info = BubbleAddExtension{Wallet: btx.account.Address, Extension: addr, Success: btx.success} // TODO: or check exit codes
	case "InstallPlugin":
		addr.Workchain = int32(msg.Payload.InstallPlugin.PluginWorkchain)
		copy(addr.Address[:], msg.Payload.InstallPlugin.PluginAddress[:])
		newChild.Info = BubbleAddExtension{Wallet: btx.account.Address, Extension: addr, Success: btx.success} // TODO: or check exit codes
	case "RemovePlugin":
		addr.Workchain = int32(msg.Payload.RemovePlugin.PluginWorkchain)
		copy(addr.Address[:], msg.Payload.RemovePlugin.PluginAddress[:])
		newChild.Info = BubbleRemoveExtension{Wallet: btx.account.Address, Extension: addr, Success: btx.success} // TODO: or check exit codes
	default:
		return
	}
	newChild.Accounts = []tongo.AccountID{btx.account.Address, addr}
	b.Children = append(b.Children, &newChild)
}
