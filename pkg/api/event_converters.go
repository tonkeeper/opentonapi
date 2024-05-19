package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/opentonapi/pkg/references"

	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/api/i18n"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
)

func distinctAccounts(skip *tongo.AccountID, book addressBook, accounts ...*tongo.AccountID) []oas.AccountAddress {
	okAccounts := make([]*tongo.AccountID, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if skip != nil && *skip == *account {
			continue
		}
		if slices.Contains(okAccounts, account) {
			continue
		}
		okAccounts = append(okAccounts, account)
	}
	result := make([]oas.AccountAddress, 0, len(okAccounts))
	for _, account := range okAccounts {
		result = append(result, convertAccountAddress(*account, book))
	}
	return result
}

func convertTrace(t *core.Trace, book addressBook) oas.Trace {
	trace := oas.Trace{Transaction: convertTransaction(t.Transaction, t.AccountInterfaces, book), Interfaces: g.ToStrings(t.AccountInterfaces)}
	for _, c := range t.Children {
		trace.Children = append(trace.Children, convertTrace(c, book))
	}
	return trace
}

func (h *Handler) convertRisk(ctx context.Context, risk wallet.Risk, walletAddress tongo.AccountID) (oas.Risk, error) {
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
			preview := jettonPreview(jettonWallet.JettonAddress, meta)
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
			nftTrustType := h.convertNftTrustType(item.CollectionAddress)
			nft := convertNFT(ctx, item, h.addressBook, h.metaCache, nftTrustType)
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

// signedValue adds either + or - in front of the provided value depending on who is looking at a simple preview.
func signedValue(value string, viewer *tongo.AccountID, source, destination tongo.AccountID) string {
	if viewer == nil {
		return value
	}
	if *viewer == source {
		return fmt.Sprintf("-%s", value)
	}
	if *viewer == destination {
		return fmt.Sprintf("+%s", value)
	}
	return value
}

func (h *Handler) convertActionTonTransfer(t *bath.TonTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptTonTransferAction, oas.ActionSimplePreview, bool) {
	var spamDetected bool
	if t.Amount < int64(ton.OneTON/10) && t.Comment != nil {
		spamAction := rules.CheckAction(h.spamFilter.GetRules(), *t.Comment)
		if spamAction != rules.UnKnown && spamAction != rules.Accept {
			spamDetected = true
			if spamAction == rules.Drop {
				*t.Comment = ""
			}
		}
	}
	var action oas.OptTonTransferAction
	action.SetTo(oas.TonTransferAction{
		Amount:           t.Amount,
		Comment:          g.Opt(t.Comment),
		Recipient:        convertAccountAddress(t.Recipient, h.addressBook),
		Sender:           convertAccountAddress(t.Sender, h.addressBook),
		EncryptedComment: convertEncryptedComment(t.EncryptedComment),
	})
	if t.Refund != nil {
		action.Value.Refund.SetTo(oas.Refund{
			Type:   oas.RefundType(t.Refund.Type),
			Origin: t.Refund.Origin,
		})
	}
	value := i18n.FormatTONs(t.Amount)
	simplePreview := oas.ActionSimplePreview{
		Name: "Ton Transfer",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "tonTransferAction",
				Other: "Transferring {{.Value}}",
			},
			TemplateData: i18n.Template{
				"Value": value,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &t.Sender, &t.Recipient),
		Value:    oas.NewOptString(value),
	}
	return action, simplePreview, spamDetected
}

func (h *Handler) convertActionNftTransfer(t *bath.NftTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptNftItemTransferAction, oas.ActionSimplePreview, bool) {
	var spamDetected bool
	if t.Comment != nil {
		if spamAction := rules.CheckAction(h.spamFilter.GetRules(), *t.Comment); spamAction == rules.Drop {
			spamDetected = true
		}
	}
	var action oas.OptNftItemTransferAction
	action.SetTo(oas.NftItemTransferAction{
		Nft:              t.Nft.ToRaw(),
		Recipient:        convertOptAccountAddress(t.Recipient, h.addressBook),
		Sender:           convertOptAccountAddress(t.Sender, h.addressBook),
		Comment:          g.Opt(t.Comment),
		EncryptedComment: convertEncryptedComment(t.EncryptedComment),
	})
	simplePreview := oas.ActionSimplePreview{
		Name: "NFT Transfer",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "nftTransferAction",
				Other: "Transferring 1 NFT",
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, t.Recipient, t.Sender, &t.Nft),
		Value:    oas.NewOptString(fmt.Sprintf("1 NFT")),
	}
	return action, simplePreview, spamDetected
}

func (h *Handler) convertActionJettonTransfer(ctx context.Context, t *bath.JettonTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptJettonTransferAction, oas.ActionSimplePreview, bool) {
	var spamDetected bool
	meta := h.GetJettonNormalizedMetadata(ctx, t.Jetton)
	preview := jettonPreview(t.Jetton, meta)
	var action oas.OptJettonTransferAction
	action.SetTo(oas.JettonTransferAction{
		Amount:           g.Pointer(big.Int(t.Amount)).String(),
		Recipient:        convertOptAccountAddress(t.Recipient, h.addressBook),
		Sender:           convertOptAccountAddress(t.Sender, h.addressBook),
		Jetton:           preview,
		RecipientsWallet: t.RecipientsWallet.ToRaw(),
		SendersWallet:    t.SendersWallet.ToRaw(),
		Comment:          g.Opt(t.Comment),
		EncryptedComment: convertEncryptedComment(t.EncryptedComment),
	})
	amount := Scale(t.Amount, meta.Decimals)
	amountString := amount.String()
	amountFloat, _ := amount.Float64()
	rates, err := h.ratesSource.GetRates(time.Now().Unix())
	if t.Comment != nil && (err == nil && amountFloat*rates[t.Jetton.ToRaw()] < 1) {
		if spamAction := rules.CheckAction(h.spamFilter.GetRules(), *t.Comment); spamAction == rules.Drop {
			spamDetected = true
		}
	}

	simplePreview := oas.ActionSimplePreview{
		Name: "Jetton Transfer",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "jettonTransferAction",
				Other: "Transferring {{.Value}} {{.JettonName}}",
			},
			TemplateData: i18n.Template{
				"Value":      amountString,
				"JettonName": meta.Name,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, t.Recipient, t.Sender, &t.Jetton),
		Value:    oas.NewOptString(fmt.Sprintf("%v %v", amountString, meta.Name)),
	}
	if len(preview.Image) > 0 {
		simplePreview.ValueImage = oas.NewOptString(preview.Image)
	}
	return action, simplePreview, spamDetected
}

func (h *Handler) convertActionJettonMint(ctx context.Context, m *bath.JettonMintAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptJettonMintAction, oas.ActionSimplePreview) {
	meta := h.GetJettonNormalizedMetadata(ctx, m.Jetton)
	preview := jettonPreview(m.Jetton, meta)
	var action oas.OptJettonMintAction
	action.SetTo(oas.JettonMintAction{
		Amount:           g.Pointer(big.Int(m.Amount)).String(),
		Recipient:        convertAccountAddress(m.Recipient, h.addressBook),
		Jetton:           preview,
		RecipientsWallet: m.RecipientsWallet.ToRaw(),
	})

	amount := Scale(m.Amount, meta.Decimals).String()
	simplePreview := oas.ActionSimplePreview{
		Name: "Jetton Mint",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "jettonMintAction",
				Other: "Minting {{.Value}} {{.JettonName}}",
			},
			TemplateData: i18n.Template{
				"Value":      amount,
				"JettonName": meta.Name,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &m.Jetton, &m.Recipient),
		Value:    oas.NewOptString(fmt.Sprintf("%v %v", amount, meta.Name)),
	}
	if len(preview.Image) > 0 {
		simplePreview.ValueImage = oas.NewOptString(preview.Image)
	}
	return action, simplePreview
}

func (h *Handler) convertActionInscriptionMint(ctx context.Context, m *bath.InscriptionMintAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptInscriptionMintAction, oas.ActionSimplePreview) {
	var action oas.OptInscriptionMintAction
	action.SetTo(oas.InscriptionMintAction{
		Recipient: convertAccountAddress(m.Minter, h.addressBook),
		Amount:    fmt.Sprintf("%v", m.Amount),
		Type:      oas.InscriptionMintActionType(m.Type),
		Ticker:    m.Ticker,
		Decimals:  9,
	})
	amount := fmt.Sprintf("%v", m.Amount/1_000_000_000)
	simplePreview := oas.ActionSimplePreview{
		Name: "Inscription Mint",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "inscriptionMintAction",
				Other: "Minting {{.Value}} {{.Ticker}}",
			},
			TemplateData: i18n.Template{
				"Value":  amount,
				"Ticker": m.Ticker,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &m.Minter),
		Value:    oas.NewOptString(fmt.Sprintf("%v %v", amount, m.Ticker)),
	}
	return action, simplePreview
}

func (h *Handler) convertActionInscriptionTransfer(ctx context.Context, t *bath.InscriptionTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptInscriptionTransferAction, oas.ActionSimplePreview) {
	var action oas.OptInscriptionTransferAction
	action.SetTo(oas.InscriptionTransferAction{
		Recipient: convertAccountAddress(t.Dst, h.addressBook),
		Sender:    convertAccountAddress(t.Src, h.addressBook),
		Amount:    fmt.Sprintf("%v", t.Amount),
		Type:      oas.InscriptionTransferActionType(t.Type),
		Ticker:    t.Ticker,
		Decimals:  9,
	})
	amount := fmt.Sprintf("%v", t.Amount/1_000_000_000)
	simplePreview := oas.ActionSimplePreview{
		Name: "Inscription Transfer",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "inscriptionTransferAction",
				Other: "Transferring {{.Value}} {{.Ticker}}",
			},
			TemplateData: i18n.Template{
				"Value":  amount,
				"Ticker": t.Ticker,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &t.Src, &t.Dst),
		Value:    oas.NewOptString(fmt.Sprintf("%v %v", amount, t.Ticker)),
	}
	return action, simplePreview
}

func (h *Handler) convertDepositStake(d *bath.DepositStakeAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptDepositStakeAction, oas.ActionSimplePreview) {
	var action oas.OptDepositStakeAction
	action.SetTo(oas.DepositStakeAction{
		Amount:         d.Amount,
		Staker:         convertAccountAddress(d.Staker, h.addressBook),
		Pool:           convertAccountAddress(d.Pool, h.addressBook),
		Implementation: oas.PoolImplementationType(d.Implementation),
	})
	simplePreview := oas.ActionSimplePreview{
		Name: "Deposit Stake",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "depositStakeAction",
				Other: "Deposit {{.Value}} to staking pool",
			},
			TemplateData: i18n.Template{
				"Value": i18n.FormatTONs(d.Amount),
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &d.Staker, &d.Pool),
		Value:    oas.NewOptString(i18n.FormatTONs(d.Amount)),
	}
	return action, simplePreview
}

func (h *Handler) convertWithdrawStakeRequest(d *bath.WithdrawStakeRequestAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptWithdrawStakeRequestAction, oas.ActionSimplePreview) {
	var action oas.OptWithdrawStakeRequestAction
	action.SetTo(oas.WithdrawStakeRequestAction{
		Amount:         g.Opt(d.Amount),
		Staker:         convertAccountAddress(d.Staker, h.addressBook),
		Pool:           convertAccountAddress(d.Pool, h.addressBook),
		Implementation: oas.PoolImplementationType(d.Implementation),
	})
	value := "ALL"
	if d.Amount != nil {
		value = i18n.FormatTONs(*d.Amount)
	}
	simplePreview := oas.ActionSimplePreview{
		Name: "Withdraw Stake Request",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "withdrawStakeRequestAction",
				Other: "Request to withdraw {{.Value}} from staking pool.",
			},
			TemplateData: i18n.Template{"Value": value},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &d.Staker, &d.Pool),
		Value:    oas.NewOptString(value),
	}
	if d.Amount != nil {
		simplePreview.Value = oas.NewOptString(i18n.FormatTONs(*d.Amount))
	}
	return action, simplePreview
}

func (h *Handler) convertWithdrawStake(d *bath.WithdrawStakeAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptWithdrawStakeAction, oas.ActionSimplePreview) {
	var action oas.OptWithdrawStakeAction
	action.SetTo(oas.WithdrawStakeAction{
		Amount:         d.Amount,
		Staker:         convertAccountAddress(d.Staker, h.addressBook),
		Pool:           convertAccountAddress(d.Pool, h.addressBook),
		Implementation: oas.PoolImplementationType(d.Implementation),
	})
	simplePreview := oas.ActionSimplePreview{
		Name: "Withdraw Stake",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "withdrawStakeAction",
				Other: "Withdraw {{.Value}} from staking pool",
			},
			TemplateData: i18n.Template{"Value": i18n.FormatTONs(d.Amount)},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &d.Staker, &d.Pool),
		Value:    oas.NewOptString(i18n.FormatTONs(d.Amount)),
	}
	return action, simplePreview
}

func (h *Handler) convertDomainRenew(ctx context.Context, d *bath.DnsRenewAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptDomainRenewAction, oas.ActionSimplePreview) {
	var action oas.OptDomainRenewAction
	var domain = "unknown"
	nfts, err := h.storage.GetNFTs(ctx, []ton.AccountID{d.Item})
	if err == nil && len(nfts) == 1 && nfts[0].DNS != nil {
		domain = *nfts[0].DNS
	}
	action.SetTo(oas.DomainRenewAction{
		Domain:          domain,
		ContractAddress: d.Item.String(),
		Renewer:         convertAccountAddress(d.Renewer, h.addressBook),
	})
	simplePreview := oas.ActionSimplePreview{
		Name: "Domain Renew",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "domainRenewAction",
				Other: "Update {{.Value}} expiring time",
			},
			TemplateData: i18n.Template{"Value": domain},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &d.Renewer, &d.Item),
		Value:    oas.NewOptString(domain),
	}
	return action, simplePreview
}

func (h *Handler) convertAction(ctx context.Context, viewer *tongo.AccountID, a bath.Action, acceptLanguage oas.OptString) (oas.Action, bool, error) {
	action := oas.Action{
		Type:             oas.ActionType(a.Type),
		BaseTransactions: make([]string, len(a.BaseTransactions)),
	}
	for i, t := range a.BaseTransactions {
		action.BaseTransactions[i] = t.Hex()
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
		action.TonTransfer, action.SimplePreview, spamDetected = h.convertActionTonTransfer(a.TonTransfer, acceptLanguage.Value, viewer)
	case bath.NftItemTransfer:
		action.NftItemTransfer, action.SimplePreview, spamDetected = h.convertActionNftTransfer(a.NftItemTransfer, acceptLanguage.Value, viewer)
	case bath.JettonTransfer:
		action.JettonTransfer, action.SimplePreview, spamDetected = h.convertActionJettonTransfer(ctx, a.JettonTransfer, acceptLanguage.Value, viewer)
	case bath.JettonMint:
		action.JettonMint, action.SimplePreview = h.convertActionJettonMint(ctx, a.JettonMint, acceptLanguage.Value, viewer)
	case bath.JettonBurn:
		meta := h.GetJettonNormalizedMetadata(ctx, a.JettonBurn.Jetton)
		preview := jettonPreview(a.JettonBurn.Jetton, meta)
		action.JettonBurn.SetTo(oas.JettonBurnAction{
			Amount:        g.Pointer(big.Int(a.JettonBurn.Amount)).String(),
			Sender:        convertAccountAddress(a.JettonBurn.Sender, h.addressBook),
			Jetton:        preview,
			SendersWallet: a.JettonBurn.SendersWallet.ToRaw(),
		})
		if len(preview.Image) > 0 {
			action.SimplePreview.ValueImage = oas.NewOptString(preview.Image)
		}
		amount := Scale(a.JettonBurn.Amount, meta.Decimals).String()
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Jetton Burn",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "jettonBurnAction",
					Other: "Burning {{.Value}} {{.JettonName}}",
				},
				TemplateData: i18n.Template{
					"Value":      amount,
					"JettonName": meta.Name,
				},
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.JettonBurn.Sender, &a.JettonBurn.Jetton),
			Value:    oas.NewOptString(fmt.Sprintf("%v %v", amount, meta.Name)),
		}
	case bath.InscriptionMint:
		action.InscriptionMint, action.SimplePreview = h.convertActionInscriptionMint(ctx, a.InscriptionMint, acceptLanguage.Value, viewer)
	case bath.InscriptionTransfer:
		action.InscriptionTransfer, action.SimplePreview = h.convertActionInscriptionTransfer(ctx, a.InscriptionTransfer, acceptLanguage.Value, viewer)
	case bath.Subscription:
		action.Subscribe.SetTo(oas.SubscriptionAction{
			Amount:       a.Subscription.Amount,
			Beneficiary:  convertAccountAddress(a.Subscription.Beneficiary, h.addressBook),
			Subscriber:   convertAccountAddress(a.Subscription.Subscriber, h.addressBook),
			Subscription: a.Subscription.Subscription.ToRaw(),
			Initial:      a.Subscription.First,
		})
		value := i18n.FormatTONs(a.Subscription.Amount)
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Subscription",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "subscriptionAction",
					Other: "Paying {{.Value}} for subscription",
				},
				TemplateData: i18n.Template{
					"Value": value,
				},
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.Subscription.Beneficiary, &a.Subscription.Subscriber),
			Value:    oas.NewOptString(value),
		}
	case bath.UnSubscription:
		action.UnSubscribe.SetTo(oas.UnSubscriptionAction{
			Beneficiary:  convertAccountAddress(a.UnSubscription.Beneficiary, h.addressBook),
			Subscriber:   convertAccountAddress(a.UnSubscription.Subscriber, h.addressBook),
			Subscription: a.UnSubscription.Subscription.ToRaw(),
		})
	case bath.ContractDeploy:
		interfaces := make([]string, 0, len(a.ContractDeploy.Interfaces))
		for _, iface := range a.ContractDeploy.Interfaces {
			interfaces = append(interfaces, iface.String())
		}
		action.ContractDeploy.SetTo(oas.ContractDeployAction{
			Address:    a.ContractDeploy.Address.ToRaw(),
			Interfaces: interfaces,
		})
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Contract Deploy",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "contractDeployAction",
					Other: "Deploying a contract{{ if .Interfaces }} with interfaces {{.Interfaces}}{{ end }}",
				},
				TemplateData: i18n.Template{
					"Interfaces": strings.Join(interfaces, ", "),
				},
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.ContractDeploy.Address),
		}
	case bath.NftPurchase:
		price := a.NftPurchase.Price
		value := i18n.FormatTONs(price)
		items, err := h.storage.GetNFTs(ctx, []tongo.AccountID{a.NftPurchase.Nft})
		if err != nil {
			return oas.Action{}, false, err
		}
		var nft oas.NftItem
		var nftImage string
		var name string
		if len(items) == 1 {
			// opentonapi doesn't implement GetNFTs() now
			nftTrustType := h.convertNftTrustType(items[0].CollectionAddress)
			nft = convertNFT(ctx, items[0], h.addressBook, h.metaCache, nftTrustType)
			if len(nft.Previews) > 0 {
				nftImage = nft.Previews[0].URL
			}
			name = optionalFromMeta(nft.Metadata, "name")
		}
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "NFT Purchase",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "nftPurchaseAction",
					Other: "Purchase {{.Name}}",
				},
				TemplateData: i18n.Template{
					"Name": name,
				},
			}),
			Accounts:   distinctAccounts(viewer, h.addressBook, &a.NftPurchase.Nft, &a.NftPurchase.Buyer),
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
	case bath.ElectionsDepositStake:
		value := i18n.FormatTONs(a.ElectionsDepositStake.Amount)
		action.ElectionsDepositStake.SetTo(oas.ElectionsDepositStakeAction{
			Amount: a.ElectionsDepositStake.Amount,
			Staker: convertAccountAddress(a.ElectionsDepositStake.Staker, h.addressBook),
		})
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Deposit Stake",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "electionsDepositStakeAction",
					Other: "Depositing {{.Amount}} for stake",
				},
				TemplateData: i18n.Template{
					"Amount": value,
				},
			}),
			Value:    oas.NewOptString(signedValue(value, viewer, a.ElectionsDepositStake.Staker, a.ElectionsDepositStake.Elector)),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.ElectionsDepositStake.Elector, &a.ElectionsDepositStake.Staker),
		}
	case bath.ElectionsRecoverStake:
		value := i18n.FormatTONs(a.ElectionsRecoverStake.Amount)
		action.ElectionsRecoverStake.SetTo(oas.ElectionsRecoverStakeAction{
			Amount: a.ElectionsRecoverStake.Amount,
			Staker: convertAccountAddress(a.ElectionsRecoverStake.Staker, h.addressBook),
		})
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Recover Stake",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "electionsRecoverStakeAction",
					Other: "Recover {{.Amount}} stake",
				},
				TemplateData: i18n.Template{
					"Amount": value,
				},
			}),
			Value:    oas.NewOptString(signedValue(value, viewer, a.ElectionsRecoverStake.Elector, a.ElectionsRecoverStake.Staker)),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.ElectionsRecoverStake.Elector, &a.ElectionsRecoverStake.Staker),
		}
	case bath.JettonSwap:
		action.Type = oas.ActionTypeJettonSwap
		swapAction := oas.JettonSwapAction{
			UserWallet: convertAccountAddress(a.JettonSwap.UserWallet, h.addressBook),
			Router:     convertAccountAddress(a.JettonSwap.Router, h.addressBook),
		}
		simplePreviewData := i18n.Template{}
		if a.JettonSwap.In.IsTon {
			swapAction.TonIn = oas.NewOptInt64(a.JettonSwap.In.Amount.Int64())
			simplePreviewData["JettonIn"] = ""
			simplePreviewData["AmountIn"] = i18n.FormatTONs(a.JettonSwap.In.Amount.Int64())
		} else {
			swapAction.AmountIn = a.JettonSwap.In.Amount.String()
			jettonInMeta := h.GetJettonNormalizedMetadata(ctx, a.JettonSwap.In.JettonMaster)
			preview := jettonPreview(a.JettonSwap.In.JettonMaster, jettonInMeta)
			swapAction.JettonMasterIn.SetTo(preview)
			simplePreviewData["JettonIn"] = preview.GetSymbol()
			simplePreviewData["AmountIn"] = ScaleJettons(a.JettonSwap.In.Amount, jettonInMeta.Decimals).String()
		}
		if a.JettonSwap.Out.IsTon {
			swapAction.TonOut = oas.NewOptInt64(a.JettonSwap.Out.Amount.Int64())
			simplePreviewData["JettonOut"] = ""
			simplePreviewData["AmountOut"] = i18n.FormatTONs(a.JettonSwap.Out.Amount.Int64())
		} else {
			swapAction.AmountOut = a.JettonSwap.Out.Amount.String()
			jettonOutMeta := h.GetJettonNormalizedMetadata(ctx, a.JettonSwap.Out.JettonMaster)
			preview := jettonPreview(a.JettonSwap.Out.JettonMaster, jettonOutMeta)
			swapAction.JettonMasterOut.SetTo(preview)
			simplePreviewData["JettonOut"] = preview.GetSymbol()
			simplePreviewData["AmountOut"] = ScaleJettons(a.JettonSwap.Out.Amount, jettonOutMeta.Decimals).String()
		}

		switch a.JettonSwap.Dex {
		case bath.Stonfi:
			swapAction.Dex = oas.JettonSwapActionDexStonfi
		case bath.Megatonfi:
			swapAction.Dex = oas.JettonSwapActionDexMegatonfi
		case bath.Dedust:
			swapAction.Dex = oas.JettonSwapActionDexDedust
		}

		action.JettonSwap.SetTo(swapAction)

		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Swap Tokens",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "jettonSwapAction",
					Other: "Swapping {{.AmountIn}} {{.JettonIn}} for {{.AmountOut}} {{.JettonOut}}",
				},
				TemplateData: simplePreviewData,
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.JettonSwap.UserWallet, &a.JettonSwap.Router),
		}
	case bath.AuctionBid:
		var nft oas.OptNftItem
		if a.AuctionBid.Nft == nil && a.AuctionBid.NftAddress != nil {
			n, err := h.storage.GetNFTs(ctx, []tongo.AccountID{*a.AuctionBid.NftAddress})
			if err != nil {
				return oas.Action{}, false, err
			}
			if len(n) == 1 {
				a.AuctionBid.Nft = &n[0]
			}
		}
		if a.AuctionBid.Nft == nil {
			return oas.Action{}, false, fmt.Errorf("nft is nil")
		}
		nftTrustType := h.convertNftTrustType(a.AuctionBid.Nft.CollectionAddress)
		nft.SetTo(convertNFT(ctx, *a.AuctionBid.Nft, h.addressBook, h.metaCache, nftTrustType))
		action.AuctionBid.SetTo(oas.AuctionBidAction{
			Amount: oas.Price{
				Value:     fmt.Sprintf("%v", a.AuctionBid.Amount),
				TokenName: "TON",
			},
			Nft:     nft,
			Bidder:  convertAccountAddress(a.AuctionBid.Bidder, h.addressBook),
			Auction: convertAccountAddress(a.AuctionBid.Auction, h.addressBook),
		})
		if a.AuctionBid.Nft.CollectionAddress != nil && *a.AuctionBid.Nft.CollectionAddress == references.RootTelegram {
			action.AuctionBid.Value.AuctionType = oas.AuctionBidActionAuctionTypeDNSTg
		} else if a.AuctionBid.Type != bath.DnsTgAuction {
			action.AuctionBid.Value.AuctionType = oas.AuctionBidActionAuctionType(a.AuctionBid.Type)
		}
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Auction bid",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "auctionBidMessage",
					Other: "Bidding {{.Amount}} for {{.NftName}}",
				},
				TemplateData: i18n.Template{
					"Amount":  i18n.FormatTONs(a.AuctionBid.Amount),
					"NftName": optionalFromMeta(nft.Value.Metadata, "name"),
				},
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.AuctionBid.Bidder, &a.AuctionBid.Auction),
		}
	case bath.SmartContractExec:
		op := "Call"
		if a.SmartContractExec.Operation != "" {
			op = a.SmartContractExec.Operation
		}
		if op == "JettonCallTo" { //todo: remove after end of april 2024
			op = "JettonAdminAction"
		}
		switch a.SmartContractExec.Operation {
		case string(bath.TfUpdateValidatorSet):
			op = "Update validator set"
		case string(bath.TfProcessPendingWithdrawRequests):
			op = "Process pending withdraw requests"
		case string(bath.TfDepositStakeRequest):
			op = "Request sending stake to Elector"
		case string(bath.TfRecoverStakeRequest):
			op = "Request Elector to recover stake"
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
				DefaultMessage: &i18n.M{
					ID:    "smartContractExecMessage",
					Other: "Execution of smart contract",
				},
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.SmartContractExec.Executor, &a.SmartContractExec.Contract),
		}
		if a.SmartContractExec.Payload != "" {
			contractAction.Payload.SetTo(a.SmartContractExec.Payload)
		}
		action.SmartContractExec.SetTo(contractAction)
	case bath.DepositStake:
		action.DepositStake, action.SimplePreview = h.convertDepositStake(a.DepositStake, acceptLanguage.Value, viewer)
	case bath.WithdrawStakeRequest:
		action.WithdrawStakeRequest, action.SimplePreview = h.convertWithdrawStakeRequest(a.WithdrawStakeRequest, acceptLanguage.Value, viewer)
	case bath.WithdrawStake:
		action.WithdrawStake, action.SimplePreview = h.convertWithdrawStake(a.WithdrawStake, acceptLanguage.Value, viewer)
	case bath.DomainRenew:
		action.DomainRenew, action.SimplePreview = h.convertDomainRenew(ctx, a.DnsRenew, acceptLanguage.Value, viewer)

	}
	return action, spamDetected, nil
}

func convertAccountValueFlow(accountID tongo.AccountID, flow *bath.AccountValueFlow, book addressBook, previews map[tongo.AccountID]oas.JettonPreview) oas.ValueFlow {
	valueFlow := oas.ValueFlow{
		Account: convertAccountAddress(accountID, book),
		Ton:     flow.Ton,
		Fees:    flow.Fees,
	}
	for jettonMaster, quantity := range flow.Jettons {
		valueFlow.Jettons = append(valueFlow.Jettons, oas.ValueFlowJettonsItem{
			Account:  convertAccountAddress(jettonMaster, book),
			Jetton:   previews[jettonMaster],
			Quantity: quantity.Int64(),
		})
	}
	return valueFlow
}

func (h *Handler) toEvent(ctx context.Context, trace *core.Trace, result *bath.ActionsList, lang oas.OptString) (oas.Event, error) {
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
		convertedAction, spamDetected, err := h.convertAction(ctx, nil, a, lang)
		if err != nil {
			return oas.Event{}, err
		}
		event.IsScam = event.IsScam || spamDetected
		event.Actions[i] = convertedAction
	}

	previews := make(map[tongo.AccountID]oas.JettonPreview)
	for _, flow := range result.ValueFlow.Accounts {
		for jettonMaster := range flow.Jettons {
			if _, ok := previews[jettonMaster]; ok {
				continue
			}
			meta := h.GetJettonNormalizedMetadata(ctx, jettonMaster)
			previews[jettonMaster] = jettonPreview(jettonMaster, meta)
		}
	}
	for accountID, flow := range result.ValueFlow.Accounts {
		event.ValueFlow = append(event.ValueFlow, convertAccountValueFlow(accountID, flow, h.addressBook, previews))
	}
	return event, nil
}

func createUnknownAction(desc string, accounts []oas.AccountAddress) oas.Action {
	return oas.Action{
		Type:   oas.ActionTypeUnknown,
		Status: oas.ActionStatusOk,
		SimplePreview: oas.ActionSimplePreview{
			Name:        "Unknown",
			Description: desc,
			Accounts:    accounts,
		},
	}
}

func (h *Handler) toAccountEventForLongTrace(account tongo.AccountID, traceID core.TraceID) oas.AccountEvent {
	e := oas.AccountEvent{
		EventID:   traceID.Hash.Hex(),
		Account:   convertAccountAddress(account, h.addressBook),
		Timestamp: traceID.UTime,
		IsScam:    false,
		Lt:        int64(traceID.Lt),
		// TODO: we don't know it InProgress if trace is long.
		InProgress: false,
		Actions: []oas.Action{
			createUnknownAction("Trace is too long.", []oas.AccountAddress{convertAccountAddress(account, h.addressBook)}),
		},
	}
	return e
}

func (h *Handler) toAccountEvent(ctx context.Context, account tongo.AccountID, trace *core.Trace, result *bath.ActionsList, lang oas.OptString, subjectOnly bool) (oas.AccountEvent, error) {
	e := oas.AccountEvent{
		EventID:    trace.Hash.Hex(),
		Account:    convertAccountAddress(account, h.addressBook),
		Timestamp:  trace.Utime,
		IsScam:     false,
		Lt:         int64(trace.Lt),
		InProgress: trace.InProgress(),
		Extra:      result.Extra(account),
	}
	for _, a := range result.Actions {
		convertedAction, spamDetected, err := h.convertAction(ctx, &account, a, lang)
		if err != nil {
			return oas.AccountEvent{}, err
		}
		if !e.IsScam && spamDetected {
			e.IsScam = true
		}
		if subjectOnly && !a.IsSubject(account) {
			continue
		}
		e.Actions = append(e.Actions, convertedAction)
	}
	if len(e.Actions) == 0 {
		e.Actions = []oas.Action{
			createUnknownAction("Something happened but we don't understand what.", []oas.AccountAddress{convertAccountAddress(account, h.addressBook)}),
		}
	}
	return e, nil
}

func convertEncryptedComment(comment *bath.EncryptedComment) oas.OptEncryptedComment {
	c := oas.OptEncryptedComment{}
	if comment != nil {
		c.SetTo(oas.EncryptedComment{EncryptionType: comment.EncryptionType, CipherText: hex.EncodeToString(comment.CipherText)})
	}
	return c
}
