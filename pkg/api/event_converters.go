package api

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/utils"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/api/i18n"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
)

// IDs of messages in pkg/i18n/translations/*.toml
const (
	tonTransferMessageID    = "tonTransferAction"
	nftTransferMessageID    = "nftTransferAction"
	nftPurchaseMessageID    = "nftPurchaseAction"
	jettonTransferMessageID = "jettonTransferAction"
	smartContractMessageID  = "smartContractExecAction"
	subscriptionMessageID   = "subscriptionAction"
	depositStakeMessageID   = "depositStakeAction"
	recoverStakeMessageID   = "recoverStakeAction"

	tfDepositMessageID                        = "tfDepositAction"
	tfRequestWithdrawMessageID                = "tfRequestWithdrawAction"
	tfProcessPendingWithdrawRequestsMessageID = "tfProcessPendingWithdrawRequestsAction"
	tfUpdateValidatorSetMessageID             = "tfUpdateValidatorSetAction"
	tfDepositStakeRequestMessageID            = "tfDepositStakeRequestAction"
	tfRecoverStakeRequestMessageID            = "tfRecoverStakeRequestAction"
)

func distinctAccounts(book addressBook, accounts ...*tongo.AccountID) []oas.AccountAddress {
	var okAccounts []*tongo.AccountID
	for _, account := range accounts {
		if account == nil {
			continue
		}
		okAccounts = append(okAccounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].String() < accounts[j].String()
	})
	sortedAccounts := slices.Compact(accounts)
	result := make([]oas.AccountAddress, 0, len(sortedAccounts))
	for _, account := range sortedAccounts {
		result = append(result, convertAccountAddress(*account, book))
	}
	return result
}

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

func optionalFromMeta(metadata oas.NftItemMetadata, name string) string {
	value, ok := metadata[name]
	if !ok {
		return ""
	}
	return strings.Trim(string(value), `"`)
}

func (h Handler) convertAction(ctx context.Context, a bath.Action, acceptLanguage oas.OptString) (oas.Action, bool, error) {

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
		Name:        string(a.Type),
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
			Comment:   g.Opt(a.TonTransfer.Comment),
			Recipient: convertAccountAddress(a.TonTransfer.Recipient, h.addressBook),
			Sender:    convertAccountAddress(a.TonTransfer.Sender, h.addressBook),
		})
		if a.TonTransfer.Refund != nil {
			action.TonTransfer.Value.Refund.SetTo(oas.Refund{
				Type:   oas.RefundType(a.TonTransfer.Refund.Type),
				Origin: a.TonTransfer.Refund.Origin,
			})
		}
		value := utils.HumanFriendlyCoinsRepr(a.TonTransfer.Amount)
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Ton Transfer",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: tonTransferMessageID,
				TemplateData: map[string]interface{}{
					"Value": value,
				},
			}),
			Accounts: distinctAccounts(h.addressBook, &a.TonTransfer.Sender, &a.TonTransfer.Recipient),
			Value:    oas.NewOptString(value),
		}
	case bath.NftItemTransfer:
		action.NftItemTransfer.SetTo(oas.NftItemTransferAction{
			Nft:       a.NftItemTransfer.Nft.ToRaw(),
			Recipient: convertOptAccountAddress(a.NftItemTransfer.Recipient, h.addressBook),
			Sender:    convertOptAccountAddress(a.NftItemTransfer.Sender, h.addressBook),
		})
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "NFT Transfer",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: nftTransferMessageID,
			}),
			Accounts: distinctAccounts(h.addressBook, a.NftItemTransfer.Recipient, a.NftItemTransfer.Sender, &a.NftItemTransfer.Nft),
			Value:    oas.NewOptString(fmt.Sprintf("1 NFT")),
		}
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
			Comment:          g.Opt(a.JettonTransfer.Comment),
		})
		if len(preview.Image) > 0 {
			action.SimplePreview.ValueImage = oas.NewOptString(preview.Image)
		}
		amount := Scale(a.JettonTransfer.Amount, meta.Decimals).String()
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Jetton Transfer",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: jettonTransferMessageID,
				TemplateData: map[string]interface{}{
					"Value":      amount,
					"JettonName": meta.Name,
				},
			}),
			Accounts: distinctAccounts(h.addressBook, a.JettonTransfer.Recipient, a.JettonTransfer.Sender, &a.JettonTransfer.Jetton),
			Value:    oas.NewOptString(fmt.Sprintf("%v %v", amount, meta.Name)),
		}

	case bath.Subscription:
		action.Subscribe.SetTo(oas.SubscriptionAction{
			Amount:       a.Subscription.Amount,
			Beneficiary:  convertAccountAddress(a.Subscription.Beneficiary, h.addressBook),
			Subscriber:   convertAccountAddress(a.Subscription.Subscriber, h.addressBook),
			Subscription: a.Subscription.Subscription.ToRaw(),
			Initial:      a.Subscription.First,
		})
		value := utils.HumanFriendlyCoinsRepr(a.Subscription.Amount)
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Subscription",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: subscriptionMessageID,
				TemplateData: map[string]interface{}{
					"Value": value,
				},
			}),
			Accounts: distinctAccounts(h.addressBook, &a.Subscription.Beneficiary, &a.Subscription.Subscriber),
			Value:    oas.NewOptString(value),
		}
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
	case bath.NftPurchase:
		price := a.NftPurchase.Price
		value := utils.HumanFriendlyCoinsRepr(price)
		items, err := h.storage.GetNFTs(ctx, []tongo.AccountID{a.NftPurchase.Nft})
		if err != nil {
			return oas.Action{}, false, err
		}
		var nft oas.NftItem
		var nftImage string
		var name string
		if len(items) == 1 {
			// opentonapi doesn't implement GetNFTs() now
			nft = convertNFT(ctx, items[0], h.addressBook, h.previewGenerator, h.metaCache)
			if len(nft.Previews) > 0 {
				nftImage = nft.Previews[0].URL
			}
			name = optionalFromMeta(nft.Metadata, "name")
		}
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "NFT Purchase",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: nftPurchaseMessageID,
				TemplateData: map[string]interface{}{
					"Name": name,
				},
			}),
			Accounts:   distinctAccounts(h.addressBook, &a.NftPurchase.Nft, &a.NftPurchase.Buyer),
			Value:      oas.NewOptString(value),
			ValueImage: oas.NewOptString(nftImage),
		}
		action.NftPurchase.SetTo(oas.NftPurchaseAction{
			AuctionType: oas.NftPurchaseActionAuctionType(a.NftPurchase.AuctionType),
			Amount:      oas.Price{Value: fmt.Sprintf("%d", price), TokenName: "TON"},
			Nft:         nft,
			Seller:      convertAccountAddress(a.NftPurchase.Seller, h.addressBook),
			Buyer:       convertAccountAddress(a.NftPurchase.Buyer, h.addressBook),
		})
	case bath.DepositStake:
		value := utils.HumanFriendlyCoinsRepr(a.DepositStake.Amount)
		action.DepositStake.SetTo(oas.DepositStakeAction{
			Amount: a.DepositStake.Amount,
			Staker: convertAccountAddress(a.DepositStake.Staker, h.addressBook),
		})
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Deposit Stake",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: depositStakeMessageID,
				TemplateData: map[string]interface{}{
					"Amount": value,
				},
			}),
			Value:    oas.NewOptString(value),
			Accounts: distinctAccounts(h.addressBook, &a.DepositStake.Elector, &a.DepositStake.Staker),
		}
	case bath.RecoverStake:
		value := utils.HumanFriendlyCoinsRepr(a.RecoverStake.Amount)
		action.RecoverStake.SetTo(oas.RecoverStakeAction{
			Amount: a.RecoverStake.Amount,
			Staker: convertAccountAddress(a.RecoverStake.Staker, h.addressBook),
		})
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Recover Stake",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: recoverStakeMessageID,
				TemplateData: map[string]interface{}{
					"Amount": value,
				},
			}),
			Value:    oas.NewOptString(value),
			Accounts: distinctAccounts(h.addressBook, &a.RecoverStake.Elector, &a.RecoverStake.Staker),
		}
	case bath.SmartContractExec:
		op := "Call"
		if a.SmartContractExec.Operation != "" {
			op = a.SmartContractExec.Operation
		}
		messageID := smartContractMessageID
		switch a.SmartContractExec.Operation {
		case string(bath.TfDeposit):
			description := i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: tfDepositMessageID,
			})
			op = description
		case string(bath.TfRequestWithdraw):
			description := i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: tfRequestWithdrawMessageID,
			})
			op = description
		case string(bath.TfUpdateValidatorSet):
			description := i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: tfUpdateValidatorSetMessageID,
			})
			op = description
		case string(bath.TfProcessPendingWithdrawRequests):
			description := i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: tfProcessPendingWithdrawRequestsMessageID,
			})
			op = description
		case string(bath.TfDepositStakeRequest):
			description := i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: tfDepositStakeRequestMessageID,
			})
			op = description
		case string(bath.TfRecoverStakeRequest):
			description := i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: tfRecoverStakeRequestMessageID,
			})
			op = description
		}
		contractAction := oas.SmartContractAction{
			Executor:    convertAccountAddress(a.SmartContractExec.Executor, h.addressBook),
			Contract:    convertAccountAddress(a.SmartContractExec.Contract, h.addressBook),
			TonAttached: a.SmartContractExec.TonAttached,
			Operation:   op,
			Refund:      oas.OptRefund{},
		}
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Smart Contract Execution",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				MessageID: messageID,
			}),
			Accounts: distinctAccounts(h.addressBook, &a.SmartContractExec.Executor, &a.SmartContractExec.Contract),
		}
		if a.SmartContractExec.Payload != "" {
			contractAction.Payload.SetTo(a.SmartContractExec.Payload)
		}
		action.SmartContractExec.SetTo(contractAction)
	}
	return action, spamDetected, nil
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

func (h Handler) toEvent(ctx context.Context, trace *core.Trace, result *bath.ActionsList, lang oas.OptString) (oas.Event, error) {
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
		convertedAction, spamDetected, err := h.convertAction(ctx, a, lang)
		if err != nil {
			return oas.Event{}, err
		}
		event.IsScam = event.IsScam || spamDetected
		event.Actions[i] = convertedAction
	}
	for accountID, flow := range result.ValueFlow.Accounts {
		event.ValueFlow = append(event.ValueFlow, convertAccountValueFlow(accountID, flow, h.addressBook))
	}
	return event, nil
}

func (h Handler) toAccountEvent(ctx context.Context, account tongo.AccountID, trace *core.Trace, result *bath.ActionsList, lang oas.OptString) (oas.AccountEvent, error) {
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
		convertedAction, spamDetected, err := h.convertAction(ctx, a, lang)
		if err != nil {
			return oas.AccountEvent{}, err
		}
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
	return e, nil
}
