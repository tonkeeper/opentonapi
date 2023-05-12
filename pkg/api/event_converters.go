package api

import (
	"context"
	"math/big"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
)

func convertTrace(t core.Trace, book addressBook) oas.Trace {
	trace := oas.Trace{Transaction: convertTransaction(t.Transaction, book), Interfaces: g.ToStrings(t.AccountInterfaces)}
	for _, c := range t.Children {
		trace.Children = append(trace.Children, convertTrace(*c, book))
	}
	return trace
}

func (h Handler) convertAction(ctx context.Context, a bath.Action) (oas.Action, bool) {
	action := oas.Action{
		Type: oas.ActionType(a.Type),
	}
	var spamDetected bool
	if a.Success {
		action.Status = oas.ActionStatusOk
	} else {
		action.Status = oas.ActionStatusFailed
	}
	switch a.Type {
	case bath.TonTransfer:
		if a.TonTransfer.Comment != nil {
			spamAction := rules.CheckAction(h.spamRules(), *a.TonTransfer.Comment)
			if spamAction == rules.Drop {
				*a.TonTransfer.Comment = ""
				spamDetected = true
			}
		}
		action.TonTransfer.SetTo(oas.TonTransferAction{
			Amount:    a.TonTransfer.Amount,
			Comment:   pointerToOptString(a.TonTransfer.Comment),
			Recipient: convertAccountAddress(a.TonTransfer.Recipient, h.addressBook),
			Sender:    convertAccountAddress(a.TonTransfer.Sender, h.addressBook),
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
			Recipient: convertOptAccountAddress(a.NftItemTransfer.Recipient, h.addressBook),
			Sender:    convertOptAccountAddress(a.NftItemTransfer.Sender, h.addressBook),
		})
	case bath.JettonTransfer:
		meta, _ := h.metaCache.getJettonMeta(ctx, a.JettonTransfer.Jetton)
		action.JettonTransfer.SetTo(oas.JettonTransferAction{
			Amount:           g.Pointer(big.Int(a.JettonTransfer.Amount)).String(),
			Recipient:        convertOptAccountAddress(a.JettonTransfer.Recipient, h.addressBook),
			Sender:           convertOptAccountAddress(a.JettonTransfer.Sender, h.addressBook),
			Jetton:           jettonPreview(h.addressBook, a.JettonTransfer.Jetton, meta, h.previewGenerator),
			RecipientsWallet: a.JettonTransfer.RecipientsWallet.ToRaw(),
			SendersWallet:    a.JettonTransfer.SendersWallet.ToRaw(),
			Comment:          pointerToOptString(a.JettonTransfer.Comment),
		})
	case bath.Subscription:
		action.Subscribe.SetTo(oas.SubscriptionAction{
			Amount:       a.Subscription.Amount,
			Beneficiary:  convertAccountAddress(a.Subscription.Beneficiary, h.addressBook),
			Subscriber:   convertAccountAddress(a.Subscription.Subscriber, h.addressBook),
			Subscription: a.Subscription.Subscription.ToRaw(),
			Initial:      a.Subscription.First,
		})
	case bath.UnSubscription:
		action.UnSubscribe.SetTo(oas.UnSubscriptionAction{
			Beneficiary:  convertAccountAddress(a.UnSubscription.Beneficiary, h.addressBook),
			Subscriber:   convertAccountAddress(a.UnSubscription.Subscriber, h.addressBook),
			Subscription: a.UnSubscription.Subscription.ToRaw(),
		})
	case bath.ContractDeploy:
		action.ContractDeploy.SetTo(oas.ContractDeployAction{
			Address:    a.ContractDeploy.Address.ToRaw(),
			Interfaces: a.ContractDeploy.Interfaces,
			Deployer:   convertAccountAddress(a.ContractDeploy.Sender, h.addressBook),
		})
	case bath.SmartContractExec:
		op := "Call"
		if a.SmartContractExec.Operation != "" {
			op = a.SmartContractExec.Operation
		}
		contractAction := oas.SmartContractAction{
			Executor:    convertAccountAddress(a.SmartContractExec.Executor, h.addressBook),
			Contract:    convertAccountAddress(a.SmartContractExec.Contract, h.addressBook),
			TonAttached: a.SmartContractExec.TonAttached,
			Operation:   op,
			Refund:      oas.OptRefund{},
		}
		if a.SmartContractExec.Payload != "" {
			contractAction.Payload.SetTo(a.SmartContractExec.Payload)
		}
		action.SmartContractExec.SetTo(contractAction)
	}
	return action, spamDetected
}

func convertAccountValueFlow(accountID tongo.AccountID, flow *bath.AccountValueFlow, book addressBook) oas.ValueFlow {
	valueFlow := oas.ValueFlow{
		Account: convertAccountAddress(accountID, book),
		Ton:     flow.Ton,
		Fees:    flow.Fees,
	}
	for nftItem, quantity := range flow.Nfts {
		valueFlow.Nfts = append(valueFlow.Nfts, oas.ValueFlowNftsItem{
			Account:  convertAccountAddress(nftItem, book),
			Quantity: quantity,
		})
	}
	for jettonItem, quantity := range flow.Jettons {
		valueFlow.Jettons = append(valueFlow.Jettons, oas.ValueFlowJettonsItem{
			Account:  convertAccountAddress(jettonItem, book),
			Quantity: quantity.Int64(),
		})
	}
	return valueFlow
}
