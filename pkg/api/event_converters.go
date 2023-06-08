package api

import (
	"context"
	"math/big"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/i18n"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
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

func (h Handler) convertRisk(ctx context.Context, risk wallet.Risk, walletAddress tongo.AccountID) (oas.Risk, error) {
	oasRisk := oas.Risk{
		TransferAllRemainingBalance: risk.TransferAllRemainingBalance,
		// TODO: verify there is no overflow
		Ton:     int64(risk.Ton),
		Jettons: nil,
		Nfts:    nil,
	}
	if len(risk.Jettons) > 0 {
		wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, walletAddress)
		if err != nil {
			return oas.Risk{}, err
		}
		for _, jettonWallet := range wallets {
			quantity, ok := risk.Jettons[jettonWallet.Address]
			if !ok {
				continue
			}
			meta := h.GetJettonNormalizedMetadata(ctx, jettonWallet.JettonAddress)
			preview := jettonPreview(jettonWallet.JettonAddress, meta, h.previewGenerator)
			jettonQuantity := oas.JettonQuantity{
				Quantity:      quantity.String(),
				WalletAddress: convertAccountAddress(jettonWallet.Address, h.addressBook),
				Jetton:        preview,
			}
			oasRisk.Jettons = append(oasRisk.Jettons, jettonQuantity)
		}
	}
	if len(risk.Nfts) > 0 {
		items, err := h.storage.GetNFTs(ctx, risk.Nfts)
		if err != nil {
			return oas.Risk{}, err
		}
		for _, item := range items {
			nft := convertNFT(ctx, item, h.addressBook, h.previewGenerator, h.metaCache)
			oasRisk.Nfts = append(oasRisk.Nfts, nft)
		}
	}
	return oasRisk, nil
}

func (h Handler) convertAction(ctx context.Context, a bath.Action, acceptLanguage oas.OptString) (oas.Action, bool) {
	action := oas.Action{
		Type: oas.ActionType(a.Type),
	}
	var spamDetected bool
	if a.Success {
		action.Status = oas.ActionStatusOk
	} else {
		action.Status = oas.ActionStatusFailed
	}

	action.SimplePreview = oas.ActionSimplePreview{
		Name:        a.SimplePreview.Name,
		Description: string(a.Type),
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
		meta := h.GetJettonNormalizedMetadata(ctx, a.JettonTransfer.Jetton)
		preview := jettonPreview(a.JettonTransfer.Jetton, meta, h.previewGenerator)
		action.JettonTransfer.SetTo(oas.JettonTransferAction{
			Amount:           g.Pointer(big.Int(a.JettonTransfer.Amount)).String(),
			Recipient:        convertOptAccountAddress(a.JettonTransfer.Recipient, h.addressBook),
			Sender:           convertOptAccountAddress(a.JettonTransfer.Sender, h.addressBook),
			Jetton:           preview,
			RecipientsWallet: a.JettonTransfer.RecipientsWallet.ToRaw(),
			SendersWallet:    a.JettonTransfer.SendersWallet.ToRaw(),
			Comment:          pointerToOptString(a.JettonTransfer.Comment),
		})
		if len(preview.Image) > 0 {
			action.SimplePreview.ValueImage = oas.NewOptString(preview.Image)
		}
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
	if len(a.SimplePreview.MessageID) > 0 {
		action.SimplePreview.Description = i18n.T(acceptLanguage.Value,
			i18n.C{
				MessageID:    a.SimplePreview.MessageID,
				TemplateData: a.SimplePreview.TemplateData,
			})
	}
	accounts := make([]oas.AccountAddress, 0, len(a.SimplePreview.Accounts))
	for _, account := range a.SimplePreview.Accounts {
		accounts = append(accounts, convertAccountAddress(account, h.addressBook))
	}
	action.SimplePreview.SetAccounts(accounts)
	if len(a.SimplePreview.Value) > 0 {
		action.SimplePreview.Value = oas.NewOptString(a.SimplePreview.Value)
	}
	return action, spamDetected
}

func convertAccountValueFlow(accountID tongo.AccountID, flow *bath.AccountValueFlow, book addressBook) oas.ValueFlow {
	valueFlow := oas.ValueFlow{
		Account: convertAccountAddress(accountID, book),
		Ton:     flow.Ton,
		Fees:    flow.Fees,
	}
	for jettonItem, quantity := range flow.Jettons {
		valueFlow.Jettons = append(valueFlow.Jettons, oas.ValueFlowJettonsItem{
			Account:  convertAccountAddress(jettonItem, book),
			Quantity: quantity.Int64(),
		})
	}
	return valueFlow
}

func (h Handler) toEvent(ctx context.Context, trace *core.Trace, result *bath.ActionsList, lang oas.OptString) oas.Event {
	event := oas.Event{
		EventID:    trace.Hash.Hex(),
		Timestamp:  trace.Utime,
		Actions:    make([]oas.Action, len(result.Actions)),
		ValueFlow:  make([]oas.ValueFlow, 0, len(result.ValueFlow.Accounts)),
		IsScam:     false,
		Lt:         int64(trace.Lt),
		InProgress: trace.InProgress(),
	}
	for i, a := range result.Actions {
		convertedAction, spamDetected := h.convertAction(ctx, a, lang)
		event.IsScam = event.IsScam || spamDetected
		event.Actions[i] = convertedAction
	}
	for accountID, flow := range result.ValueFlow.Accounts {
		event.ValueFlow = append(event.ValueFlow, convertAccountValueFlow(accountID, flow, h.addressBook))
	}
	return event
}

func (h Handler) toAccountEvent(ctx context.Context, account tongo.AccountID, trace *core.Trace, result *bath.ActionsList, lang oas.OptString) oas.AccountEvent {
	e := oas.AccountEvent{
		EventID:    trace.Hash.Hex(),
		Account:    convertAccountAddress(account, h.addressBook),
		Timestamp:  trace.Utime,
		Fee:        oas.Fee{Account: convertAccountAddress(account, h.addressBook)},
		IsScam:     false,
		Lt:         int64(trace.Lt),
		InProgress: trace.InProgress(),
		Extra:      result.Extra(account, trace),
	}
	for _, a := range result.Actions {
		convertedAction, spamDetected := h.convertAction(ctx, a, lang)
		if !e.IsScam && spamDetected {
			e.IsScam = true
		}
		e.Actions = append(e.Actions, convertedAction)
	}
	if len(e.Actions) == 0 {
		e.Actions = []oas.Action{{
			Type:   oas.ActionTypeUnknown,
			Status: oas.ActionStatusOk,
			SimplePreview: oas.ActionSimplePreview{
				Name:        "Unknown",
				Description: "Something happened but we don't understand what.",
				Accounts:    []oas.AccountAddress{convertAccountAddress(account, h.addressBook)},
			},
		}}
	}
	return e
}
