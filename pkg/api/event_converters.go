package api

import (
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertTrace(t core.Trace, book addressBook) oas.Trace {
	trace := oas.Trace{Transaction: convertTransaction(t.Transaction, book)}
	for _, c := range t.Children {
		trace.Children = append(trace.Children, convertTrace(*c, book))
	}
	return trace
}

func convertAction(a bath.Action, book addressBook) oas.Action {

	action := oas.Action{
		Type: oas.ActionType(a.Type),
	}
	if a.Success {
		action.Status = oas.ActionStatusOk
	} else {
		action.Status = oas.ActionStatusFailed
	}
	switch a.Type {
	case bath.TonTransfer:
		action.TonTransfer.SetTo(oas.TonTransferAction{
			Amount:    a.TonTransfer.Amount,
			Comment:   pointerToOptString(a.TonTransfer.Comment),
			Recipient: convertAccountAddress(a.TonTransfer.Recipient, book),
			Sender:    convertAccountAddress(a.TonTransfer.Sender, book),
		})
		if a.TonTransfer.Refund != nil {
			action.TonTransfer.Value.Refund.SetTo(oas.Refund{
				Type:   oas.RefundType(a.TonTransfer.Refund.Type),
				Origin: a.TonTransfer.Refund.Origin,
			})

		}
	case bath.NftItemTransfer:
		action.NftItemTransfer.SetTo(oas.NftItemTransferAction{
			Nft:       a.NftItemTransfer.Nft.ToRaw(),
			Recipient: convertOptAccountAddress(a.NftItemTransfer.Recipient, book),
			Sender:    convertOptAccountAddress(a.NftItemTransfer.Sender, book),
		})
	case bath.JettonTransfer:
		action.JettonTransfer.SetTo(oas.JettonTransferAction{
			//Amount:           a.JettonTransfer.Amount.String(),
			Recipient:        convertOptAccountAddress(a.JettonTransfer.Recipient, book),
			Sender:           convertOptAccountAddress(a.JettonTransfer.Sender, book),
			RecipientsWallet: a.JettonTransfer.RecipientsWallet.ToRaw(),
			SendersWallet:    a.JettonTransfer.SendersWallet.ToRaw(),
			Comment:          pointerToOptString(a.JettonTransfer.Comment),
		})
	case bath.Subscription:
		action.Subscribe.SetTo(oas.SubscriptionAction{
			Amount:       a.Subscription.Amount,
			Beneficiary:  convertAccountAddress(a.Subscription.Beneficiary, book),
			Subscriber:   convertAccountAddress(a.Subscription.Subscriber, book),
			Subscription: a.Subscription.Subscription.ToRaw(),
			Initial:      a.Subscription.First,
		})
	case bath.UnSubscription:
		action.UnSubscribe.SetTo(oas.UnSubscriptionAction{
			Beneficiary:  convertAccountAddress(a.UnSubscription.Beneficiary, book),
			Subscriber:   convertAccountAddress(a.UnSubscription.Subscriber, book),
			Subscription: a.UnSubscription.Subscription.ToRaw(),
		})
	case bath.ContractDeploy:
		action.ContractDeploy.SetTo(oas.ContractDeployAction{
			Address:    a.ContractDeploy.Address.ToRaw(),
			Interfaces: a.ContractDeploy.Interfaces,
			Deployer:   convertAccountAddress(a.ContractDeploy.Sender, book),
		})
	case bath.SmartContractExec:
		op := "Call"
		if a.SmartContractExec.Operation != "" {
			op = a.SmartContractExec.Operation
		}
		contractAction := oas.SmartContractAction{
			Executor:    convertAccountAddress(a.SmartContractExec.Executor, book),
			Contract:    convertAccountAddress(a.SmartContractExec.Contract, book),
			TonAttached: a.SmartContractExec.TonAttached,
			Operation:   op,
			Refund:      oas.OptRefund{},
		}
		if a.SmartContractExec.Payload != "" {
			contractAction.Payload.SetTo(a.SmartContractExec.Payload)
		}
		action.SmartContractExec.SetTo(contractAction)
	}
	return action
}

func convertFees(fee bath.Fee, book addressBook) oas.Fee {
	return oas.Fee{
		Account: convertAccountAddress(fee.WhoPay, book),
		Total:   0,
		Gas:     fee.Compute,
		Rent:    fee.Storage,
		Deposit: fee.Deposit,
		Refund:  0,
	}
}
