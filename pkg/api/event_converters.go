package api

import (
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertTrace(t core.Trace) oas.Trace {
	trace := oas.Trace{Transaction: convertTransaction(t.Transaction)}
	for _, c := range t.Children {
		trace.Children = append(trace.Children, convertTrace(*c))
	}
	return trace
}

func convertAction(a bath.Action) oas.Action {

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
			Payload:   pointerToOptString(a.TonTransfer.Payload),
			Recipient: convertAccountAddress(a.TonTransfer.Recipient),
			Sender:    convertAccountAddress(a.TonTransfer.Sender),
		})
		if a.TonTransfer.Refund != nil {
			action.TonTransfer.Value.Refund.SetTo(oas.Refund{
				Type:   oas.RefundType(a.TonTransfer.Refund.Type),
				Origin: a.TonTransfer.Refund.Origin,
			})

		}

	case bath.NftTransfer:
		action.NftItemTransfer.SetTo(oas.NftItemTransferAction{
			Nft:       a.NftTransfer.Nft.ToRaw(),
			Recipient: convertOptAccountAddress(a.NftTransfer.Recipient),
			Sender:    convertOptAccountAddress(a.NftTransfer.Sender),
		})
	case bath.JettonTransfer:
		action.JettonTransfer.SetTo(oas.JettonTransferAction{
			Amount:           a.JettonTransfer.Amount.String(),
			Recipient:        convertOptAccountAddress(a.JettonTransfer.Recipient),
			Sender:           convertOptAccountAddress(a.JettonTransfer.Sender),
			RecipientsWallet: a.JettonTransfer.RecipientsWallet.ToRaw(),
			SendersWallet:    a.JettonTransfer.SendersWallet.ToRaw(),
			Comment:          pointerToOptString(a.JettonTransfer.Comment),
		})
	case bath.Subscription:
		action.Subscribe.SetTo(oas.SubscriptionAction{
			Amount:       a.Subscription.Amount,
			Beneficiary:  convertAccountAddress(a.Subscription.Beneficiary),
			Subscriber:   convertAccountAddress(a.Subscription.Subscriber),
			Subscription: a.Subscription.Subscription.ToRaw(),
			Initial:      a.Subscription.First,
		})
	case bath.UnSubscription:
		action.UnSubscribe.SetTo(oas.UnSubscriptionAction{
			Beneficiary:  convertAccountAddress(a.UnSubscription.Beneficiary),
			Subscriber:   convertAccountAddress(a.UnSubscription.Subscriber),
			Subscription: a.UnSubscription.Subscription.ToRaw(),
		})
	case bath.ContractDeploy:
		action.ContractDeploy.SetTo(oas.ContractDeployAction{
			Address:    a.ContractDeploy.Address.ToRaw(),
			Interfaces: a.ContractDeploy.Interfaces,
			Deployer:   convertAccountAddress(a.ContractDeploy.Sender),
		})

	}
	return action
}

func convertFees(fee bath.Fee) oas.Fee {
	return oas.Fee{
		Account: convertAccountAddress(fee.WhoPay),
		Total:   0,
		Gas:     fee.Compute,
		Rent:    fee.Storage,
		Deposit: fee.Deposit,
		Refund:  0,
	}
}
