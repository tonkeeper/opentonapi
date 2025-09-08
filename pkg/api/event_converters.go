package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"go.uber.org/zap"

	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/opentonapi/pkg/references"

	"github.com/tonkeeper/tongo"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/api/i18n"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
)

var unknownEventCounterVec = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "unknown_account_event_method_called_total",
		Help: "Total number of times toUnknownAccountEvent method was called",
	},
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
	trace := oas.Trace{
		Transaction: convertTransaction(t.Transaction, t.AccountInterfaces, book),
		Interfaces:  g.ToStrings(t.AccountInterfaces),
		Emulated:    oas.OptBool{Set: true, Value: t.Emulated},
	}

	sort.Slice(t.Children, func(i, j int) bool {
		if t.Children[i].InMsg == nil || t.Children[j].InMsg == nil {
			return false
		}
		return t.Children[i].InMsg.CreatedLt < t.Children[j].InMsg.CreatedLt
	})
	for _, c := range t.Children {
		trace.Children = append(trace.Children, convertTrace(c, book))
	}
	return trace
}

func (h *Handler) convertRisk(ctx context.Context, risk wallet.Risk, walletAddress tongo.AccountID) (oas.Risk, error) {
	if int64(risk.Ton) < 0 {
		return oas.Risk{}, fmt.Errorf("ivalid ton amount")
	}
	oasRisk := oas.Risk{
		TransferAllRemainingBalance: risk.TransferAllRemainingBalance,
		Ton:                         int64(risk.Ton),
		Jettons:                     nil,
		Nfts:                        nil,
	}
	for jetton, quantity := range risk.Jettons {
		jettonWallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, walletAddress, &jetton, false, true)
		if err != nil || len(jettonWallets) == 0 {
			continue
		}
		jettonWallet := jettonWallets[0]
		meta := h.GetJettonNormalizedMetadata(ctx, jettonWallet.JettonAddress)
		score, _ := h.score.GetJettonScore(jettonWallet.JettonAddress)
		preview := jettonPreview(jettonWallet.JettonAddress, meta, score)
		jettonQuantity := oas.JettonQuantity{
			Quantity:      quantity.String(),
			WalletAddress: convertAccountAddress(jettonWallet.Address, h.addressBook),
			Jetton:        preview,
		}
		oasRisk.Jettons = append(oasRisk.Jettons, jettonQuantity)
	}
	if len(risk.Nfts) > 0 {
		var wg sync.WaitGroup
		wg.Add(1)
		var nftsScamData map[ton.AccountID]core.TrustType
		var err error
		go func() {
			defer wg.Done()
			nftsScamData, err = h.spamFilter.GetNftsScamData(ctx, risk.Nfts)
			if err != nil {
				h.logger.Warn("error getting nft scam data", zap.Error(err))
			}
		}()
		items, err := h.storage.GetNFTs(ctx, risk.Nfts)
		wg.Wait()
		if err != nil {
			return oas.Risk{}, err
		}
		for _, item := range items {
			nft := h.convertNFT(ctx, item, h.addressBook, h.metaCache, nftsScamData[item.Address])
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

func (h *Handler) convertActionTonTransfer(t *bath.TonTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptTonTransferAction, oas.ActionSimplePreview) {

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
	return action, simplePreview
}

func (h *Handler) convertActionExtraCurrencyTransfer(t *bath.ExtraCurrencyTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptExtraCurrencyTransferAction, oas.ActionSimplePreview) {
	var action oas.OptExtraCurrencyTransferAction
	amount := big.Int(t.Amount)
	meta := references.GetExtraCurrencyMeta(t.CurrencyID)
	action.SetTo(oas.ExtraCurrencyTransferAction{
		Amount:           amount.String(),
		Comment:          g.Opt(t.Comment),
		Recipient:        convertAccountAddress(t.Recipient, h.addressBook),
		Sender:           convertAccountAddress(t.Sender, h.addressBook),
		EncryptedComment: convertEncryptedComment(t.EncryptedComment),
		Currency: oas.EcPreview{
			ID:       t.CurrencyID,
			Symbol:   meta.Symbol,
			Decimals: meta.Decimals,
			Image:    meta.Image,
		},
	})
	value := i18n.FormatTokens(big.Int(t.Amount), int32(meta.Decimals), meta.Symbol)
	simplePreview := oas.ActionSimplePreview{
		Name:        "Extra Currency Transfer",
		Description: "", // TODO: add description
		Accounts:    distinctAccounts(viewer, h.addressBook, &t.Sender, &t.Recipient),
		Value:       oas.NewOptString(value),
	}
	return action, simplePreview
}

func (h *Handler) convertActionNftTransfer(t *bath.NftTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptNftItemTransferAction, oas.ActionSimplePreview) {
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
	return action, simplePreview
}

func (h *Handler) convertActionJettonTransfer(ctx context.Context, t *bath.JettonTransferAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptJettonTransferAction, oas.ActionSimplePreview) {
	meta := h.GetJettonNormalizedMetadata(ctx, t.Jetton)
	score, _ := h.score.GetJettonScore(t.Jetton)
	preview := jettonPreview(t.Jetton, meta, score)
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
	value := i18n.FormatTokens(big.Int(t.Amount), int32(meta.Decimals), meta.Symbol)

	simplePreview := oas.ActionSimplePreview{
		Name: "Jetton Transfer",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "jettonTransferAction",
				Other: "Transferring {{.Value}}",
			},
			TemplateData: i18n.Template{
				"Value": value,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, t.Recipient, t.Sender, &t.Jetton),
		Value:    oas.NewOptString(value),
	}
	if len(preview.Image) > 0 {
		simplePreview.ValueImage = oas.NewOptString(preview.Image)
	}
	return action, simplePreview
}

func (h *Handler) convertActionJettonMint(ctx context.Context, m *bath.JettonMintAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptJettonMintAction, oas.ActionSimplePreview) {
	meta := h.GetJettonNormalizedMetadata(ctx, m.Jetton)
	score, _ := h.score.GetJettonScore(m.Jetton)
	preview := jettonPreview(m.Jetton, meta, score)
	var action oas.OptJettonMintAction
	action.SetTo(oas.JettonMintAction{
		Amount:           g.Pointer(big.Int(m.Amount)).String(),
		Recipient:        convertAccountAddress(m.Recipient, h.addressBook),
		Jetton:           preview,
		RecipientsWallet: m.RecipientsWallet.ToRaw(),
	})

	value := i18n.FormatTokens(big.Int(m.Amount), int32(meta.Decimals), meta.Symbol)
	simplePreview := oas.ActionSimplePreview{
		Name: "Jetton Mint",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "jettonMintAction",
				Other: "Minting {{.Value}}",
			},
			TemplateData: i18n.Template{
				"Value": value,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &m.Jetton, &m.Recipient),
		Value:    oas.NewOptString(value),
	}
	if len(preview.Image) > 0 {
		simplePreview.ValueImage = oas.NewOptString(preview.Image)
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

func (h *Handler) convertDepositTokenStake(ctx context.Context, d *bath.DepositTokenStakeAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptDepositTokenStakeAction, oas.ActionSimplePreview) {
	p := h.convertPrice(ctx, *d.StakeMeta)
	var price oas.OptPrice
	price.SetTo(p)

	var image oas.OptString
	if d.Protocol.Image != nil {
		image = oas.NewOptString(imgGenerator.DefaultGenerator.GenerateImageUrl(*d.Protocol.Image, 200, 200))
	}

	var action oas.OptDepositTokenStakeAction
	action.SetTo(oas.DepositTokenStakeAction{
		Staker: convertAccountAddress(d.Staker, h.addressBook),
		Protocol: oas.Protocol{
			Name:  d.Protocol.Name,
			Image: image,
		},
		StakeMeta: price,
	})
	simplePreview := oas.ActionSimplePreview{
		Name: "Deposit Token Stake",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "depositTokenStakeAction",
				Other: "Staked with {{.Protocol}} protocol",
			},
			TemplateData: i18n.Template{
				"Protocol": d.Protocol.Name,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &d.Staker, d.StakeMeta.Currency.Jetton),
		Value:    oas.NewOptString(i18n.FormatTokens(d.StakeMeta.Amount, int32(p.Decimals), p.TokenName)),
	}
	return action, simplePreview
}

func (h *Handler) convertWithdrawTokenStakeRequest(ctx context.Context, w *bath.WithdrawTokenStakeRequestAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptWithdrawTokenStakeRequestAction, oas.ActionSimplePreview) {
	p := h.convertPrice(ctx, *w.StakeMeta)
	var price oas.OptPrice
	price.SetTo(p)

	var image oas.OptString
	if w.Protocol.Image != nil {
		image = oas.NewOptString(imgGenerator.DefaultGenerator.GenerateImageUrl(*w.Protocol.Image, 200, 200))
	}

	var action oas.OptWithdrawTokenStakeRequestAction
	action.SetTo(oas.WithdrawTokenStakeRequestAction{
		Staker: convertAccountAddress(w.Staker, h.addressBook),
		Protocol: oas.Protocol{
			Name:  w.Protocol.Name,
			Image: image,
		},
		StakeMeta: price,
	})
	simplePreview := oas.ActionSimplePreview{
		Name: "Withdraw Token Stake",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "withdrawTokenStakeAction",
				Other: "Request to withdraw from {{.Protocol}} protocol",
			},
			TemplateData: i18n.Template{
				"Protocol": w.Protocol.Name,
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &w.Staker, w.StakeMeta.Currency.Jetton),
		Value:    oas.NewOptString("ALL"),
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

func (h *Handler) convertPurchaseAction(ctx context.Context, p *bath.PurchaseAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptPurchaseAction, oas.ActionSimplePreview) {
	price := h.convertPrice(ctx, p.Price)
	currency := ""
	switch p.Price.Currency.Type {
	case core.CurrencyJetton:
		currency = p.Price.Currency.Jetton.ToRaw()
	case core.CurrencyExtra:
		currency = fmt.Sprintf("%d", int64(uint32(*p.Price.Currency.CurrencyID))) // in db as uint32
	}
	purchaseAction := oas.PurchaseAction{
		Source:      convertAccountAddress(p.Source, h.addressBook),
		Destination: convertAccountAddress(p.Destination, h.addressBook),
		InvoiceID:   p.InvoiceID.String(),
		Amount:      price,
	}
	value := i18n.FormatTokens(p.Price.Amount, int32(price.Decimals), price.TokenName)
	simplePreview := oas.ActionSimplePreview{
		Name: "Purchase",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "purchaseAction",
				Other: "Payment for invoice #{{.InvoiceID}}",
			},
			TemplateData: i18n.Template{
				"InvoiceID": p.InvoiceID.String(),
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &p.Source, &p.Destination),
		Value:    oas.NewOptString(value),
	}
	inv, err := h.storage.GetInvoice(ctx, p.Source, p.Destination, p.InvoiceID, currency)
	if err == nil {
		meta, err := convertMetadata(inv.Metadata, nil)
		if err == nil {
			purchaseAction.Metadata = meta
		}
	}
	var action oas.OptPurchaseAction
	action.SetTo(purchaseAction)
	return action, simplePreview
}

func (h *Handler) convertAction(ctx context.Context, viewer *tongo.AccountID, a bath.Action, acceptLanguage oas.OptString) (oas.Action, error) {
	action := oas.Action{
		Type:             oas.ActionType(a.Type),
		BaseTransactions: make([]string, len(a.BaseTransactions)),
	}
	for i, t := range a.BaseTransactions {
		action.BaseTransactions[i] = t.Hex()
	}
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
		action.TonTransfer, action.SimplePreview = h.convertActionTonTransfer(a.TonTransfer, acceptLanguage.Value, viewer)
	case bath.ExtraCurrencyTransfer:
		action.ExtraCurrencyTransfer, action.SimplePreview = h.convertActionExtraCurrencyTransfer(a.ExtraCurrencyTransfer, acceptLanguage.Value, viewer)
	case bath.NftItemTransfer:
		action.NftItemTransfer, action.SimplePreview = h.convertActionNftTransfer(a.NftItemTransfer, acceptLanguage.Value, viewer)
	case bath.JettonTransfer:
		action.JettonTransfer, action.SimplePreview = h.convertActionJettonTransfer(ctx, a.JettonTransfer, acceptLanguage.Value, viewer)
	case bath.JettonMint:
		action.JettonMint, action.SimplePreview = h.convertActionJettonMint(ctx, a.JettonMint, acceptLanguage.Value, viewer)
	case bath.JettonBurn:
		meta := h.GetJettonNormalizedMetadata(ctx, a.JettonBurn.Jetton)
		score, _ := h.score.GetJettonScore(a.JettonBurn.Jetton)
		preview := jettonPreview(a.JettonBurn.Jetton, meta, score)
		action.JettonBurn.SetTo(oas.JettonBurnAction{
			Amount:        g.Pointer(big.Int(a.JettonBurn.Amount)).String(),
			Sender:        convertAccountAddress(a.JettonBurn.Sender, h.addressBook),
			Jetton:        preview,
			SendersWallet: a.JettonBurn.SendersWallet.ToRaw(),
		})
		if len(preview.Image) > 0 {
			action.SimplePreview.ValueImage = oas.NewOptString(preview.Image)
		}
		value := i18n.FormatTokens(big.Int(a.JettonBurn.Amount), int32(meta.Decimals), meta.Symbol)
		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Jetton Burn",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "jettonBurnAction",
					Other: "Burning {{.Value}}",
				},
				TemplateData: i18n.Template{
					"Value": value,
				},
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.JettonBurn.Sender, &a.JettonBurn.Jetton),
			Value:    oas.NewOptString(value),
		}
	case bath.Subscribe:
		action.Subscribe, action.SimplePreview = h.convertSubscribe(ctx, a.Subscribe, acceptLanguage.Value, viewer)
	case bath.UnSubscribe:
		action.UnSubscribe, action.SimplePreview = h.convertUnsubscribe(ctx, a.UnSubscribe, acceptLanguage.Value, viewer)
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
		price := h.convertPrice(ctx, a.NftPurchase.Price)
		value := i18n.FormatTokens(a.NftPurchase.Price.Amount, int32(price.Decimals), price.TokenName)
		items, err := h.storage.GetNFTs(ctx, []tongo.AccountID{a.NftPurchase.Nft})
		if err != nil {
			return oas.Action{}, err
		}
		var nft oas.NftItem
		var nftImage string
		var name string
		if len(items) == 1 {
			// opentonapi doesn't implement GetNFTs() now
			nft = h.convertNFT(ctx, items[0], h.addressBook, h.metaCache, "")
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
			Amount:      price,
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
			Dex:        string(a.JettonSwap.Dex),
		}
		simplePreviewData := i18n.Template{}
		if a.JettonSwap.In.IsTon {
			swapAction.TonIn = oas.NewOptInt64(a.JettonSwap.In.Amount.Int64())
			simplePreviewData["AmountIn"] = i18n.FormatTONs(a.JettonSwap.In.Amount.Int64())
		} else {
			swapAction.AmountIn = a.JettonSwap.In.Amount.String()
			jettonInMeta := h.GetJettonNormalizedMetadata(ctx, a.JettonSwap.In.JettonMaster)
			score, _ := h.score.GetJettonScore(a.JettonSwap.In.JettonMaster)
			preview := jettonPreview(a.JettonSwap.In.JettonMaster, jettonInMeta, score)
			swapAction.JettonMasterIn.SetTo(preview)
			simplePreviewData["AmountIn"] = i18n.FormatTokens(a.JettonSwap.In.Amount, int32(jettonInMeta.Decimals), jettonInMeta.Symbol)
		}
		if a.JettonSwap.Out.IsTon {
			swapAction.TonOut = oas.NewOptInt64(a.JettonSwap.Out.Amount.Int64())
			simplePreviewData["AmountOut"] = i18n.FormatTONs(a.JettonSwap.Out.Amount.Int64())
		} else {
			swapAction.AmountOut = a.JettonSwap.Out.Amount.String()
			jettonOutMeta := h.GetJettonNormalizedMetadata(ctx, a.JettonSwap.Out.JettonMaster)
			score, _ := h.score.GetJettonScore(a.JettonSwap.Out.JettonMaster)
			preview := jettonPreview(a.JettonSwap.Out.JettonMaster, jettonOutMeta, score)
			swapAction.JettonMasterOut.SetTo(preview)
			simplePreviewData["AmountOut"] = i18n.FormatTokens(a.JettonSwap.Out.Amount, int32(jettonOutMeta.Decimals), jettonOutMeta.Symbol)
		}

		action.JettonSwap.SetTo(swapAction)

		action.SimplePreview = oas.ActionSimplePreview{
			Name: "Swap Tokens",
			Description: i18n.T(acceptLanguage.Value, i18n.C{
				DefaultMessage: &i18n.M{
					ID:    "jettonSwapAction",
					Other: "Swapping {{.AmountIn}} for {{.AmountOut}}",
				},
				TemplateData: simplePreviewData,
			}),
			Accounts: distinctAccounts(viewer, h.addressBook, &a.JettonSwap.UserWallet, &a.JettonSwap.Router),
		}
	case bath.AuctionBid:
		var nft oas.OptNftItem
		price := h.convertPrice(ctx, a.AuctionBid.Amount)
		value := i18n.FormatTokens(a.AuctionBid.Amount.Amount, int32(price.Decimals), price.TokenName)
		if a.AuctionBid.Nft == nil && a.AuctionBid.NftAddress != nil {
			n, err := h.storage.GetNFTs(ctx, []tongo.AccountID{*a.AuctionBid.NftAddress})
			if err != nil {
				return oas.Action{}, err
			}
			if len(n) == 1 {
				a.AuctionBid.Nft = &n[0]
			}
		}
		if a.AuctionBid.Nft == nil {
			return oas.Action{}, fmt.Errorf("nft is nil")
		}
		nft.SetTo(h.convertNFT(ctx, *a.AuctionBid.Nft, h.addressBook, h.metaCache, ""))
		action.AuctionBid.SetTo(oas.AuctionBidAction{
			Amount:  price,
			Nft:     nft,
			Bidder:  convertAccountAddress(a.AuctionBid.Bidder, h.addressBook),
			Auction: convertAccountAddress(a.AuctionBid.Auction, h.addressBook),
		})
		if a.AuctionBid.Nft.CollectionAddress != nil && *a.AuctionBid.Nft.CollectionAddress == references.RootTelegram {
			action.AuctionBid.Value.AuctionType = oas.AuctionBidActionAuctionTypeDNSTg
		} else {
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
					"Amount":  oas.NewOptString(value),
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
	case bath.WithdrawTokenStakeRequest:
		action.WithdrawTokenStakeRequest, action.SimplePreview = h.convertWithdrawTokenStakeRequest(ctx, a.WithdrawTokenStakeRequest, acceptLanguage.Value, viewer)
	case bath.DepositTokenStake:
		action.DepositTokenStake, action.SimplePreview = h.convertDepositTokenStake(ctx, a.DepositTokenStake, acceptLanguage.Value, viewer)
	case bath.WithdrawStakeRequest:
		action.WithdrawStakeRequest, action.SimplePreview = h.convertWithdrawStakeRequest(a.WithdrawStakeRequest, acceptLanguage.Value, viewer)
	case bath.WithdrawStake:
		action.WithdrawStake, action.SimplePreview = h.convertWithdrawStake(a.WithdrawStake, acceptLanguage.Value, viewer)
	case bath.DomainRenew:
		action.DomainRenew, action.SimplePreview = h.convertDomainRenew(ctx, a.DnsRenew, acceptLanguage.Value, viewer)
	case bath.Purchase:
		action.Purchase, action.SimplePreview = h.convertPurchaseAction(ctx, a.Purchase, acceptLanguage.Value, viewer)
	case bath.AddExtension:
		action.AddExtension, action.SimplePreview = h.convertAddExtensionAction(ctx, a.AddExtension, acceptLanguage.Value, viewer)
	case bath.RemoveExtension:
		action.RemoveExtension, action.SimplePreview = h.convertRemoveExtensionAction(ctx, a.RemoveExtension, acceptLanguage.Value, viewer)
	case bath.SetSignatureAllowed:
		action.SetSignatureAllowedAction, action.SimplePreview = h.convertSetSignatureAllowed(ctx, a.SetSignatureAllowed, acceptLanguage.Value, viewer)
	case bath.GasRelay:
		action.GasRelay, action.SimplePreview = h.convertGasRelayAction(a.GasRelay, acceptLanguage.Value, viewer)
	}
	return action, nil
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
			Qty:      quantity.String(),
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
		Progress:   trace.CalculateProgress(),
	}
	for i, a := range result.Actions {
		convertedAction, err := h.convertAction(ctx, nil, a, lang)
		if err != nil {
			return oas.Event{}, err
		}
		event.Actions[i] = convertedAction
	}
	event.IsScam = h.spamFilter.IsScamEvent(event.Actions, nil, trace.Account)
	previews := make(map[tongo.AccountID]oas.JettonPreview)
	for _, flow := range result.ValueFlow.Accounts {
		for jettonMaster := range flow.Jettons {
			if _, ok := previews[jettonMaster]; ok {
				continue
			}
			meta := h.GetJettonNormalizedMetadata(ctx, jettonMaster)
			score, _ := h.score.GetJettonScore(jettonMaster)
			previews[jettonMaster] = jettonPreview(jettonMaster, meta, score)
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
			createUnknownAction("Event is too big.", []oas.AccountAddress{convertAccountAddress(account, h.addressBook)}),
		},
	}
	return e
}

func (h *Handler) toUnknownAccountEvent(account tongo.AccountID, traceID core.TraceID) oas.AccountEvent {
	unknownEventCounterVec.Inc()
	slog.Error(
		"failed to get account event",
		slog.String("eventID", traceID.Hash.Hex()),
	)
	e := oas.AccountEvent{
		EventID:    traceID.Hash.Hex(),
		Account:    convertAccountAddress(account, h.addressBook),
		Timestamp:  traceID.UTime,
		IsScam:     false,
		Lt:         int64(traceID.Lt),
		InProgress: false,
		Actions: []oas.Action{
			createUnknownAction("Some error in indexer", []oas.AccountAddress{convertAccountAddress(account, h.addressBook)}),
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
		Progress:   trace.CalculateProgress(),
	}
	for _, a := range result.Actions {
		if subjectOnly && !a.IsSubject(account) {
			continue
		}
		convertedAction, err := h.convertAction(ctx, &account, a, lang)
		if err != nil {
			return oas.AccountEvent{}, err
		}
		e.Actions = append(e.Actions, convertedAction)
	}
	if h.spamFilter != nil {
		e.IsScam = h.spamFilter.IsScamEvent(e.Actions, &account, trace.Account)
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

func (h *Handler) convertAddExtensionAction(ctx context.Context, p *bath.AddExtensionAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptAddExtensionAction, oas.ActionSimplePreview) {
	addExtensionAction := oas.AddExtensionAction{
		Wallet:    convertAccountAddress(p.Wallet, h.addressBook),
		Extension: p.Extension.ToRaw(),
	}
	simplePreview := oas.ActionSimplePreview{
		Name: "AddExtension",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "addExtensionAction",
				Other: "Add extension to wallet",
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &p.Wallet, &p.Extension),
	}
	var action oas.OptAddExtensionAction
	action.SetTo(addExtensionAction)
	return action, simplePreview
}

func (h *Handler) convertRemoveExtensionAction(ctx context.Context, p *bath.RemoveExtensionAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptRemoveExtensionAction, oas.ActionSimplePreview) {
	removeExtensionAction := oas.RemoveExtensionAction{
		Wallet:    convertAccountAddress(p.Wallet, h.addressBook),
		Extension: p.Extension.ToRaw(),
	}
	simplePreview := oas.ActionSimplePreview{
		Name: "RemoveExtension",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "removeExtensionAction",
				Other: "Remove extension from wallet",
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &p.Wallet, &p.Extension),
	}
	var action oas.OptRemoveExtensionAction
	action.SetTo(removeExtensionAction)
	return action, simplePreview
}

func (h *Handler) convertSetSignatureAllowed(ctx context.Context, p *bath.SetSignatureAllowedAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptSetSignatureAllowedAction, oas.ActionSimplePreview) {
	setSignatureAllowedAction := oas.SetSignatureAllowedAction{
		Wallet:  convertAccountAddress(p.Wallet, h.addressBook),
		Allowed: p.SignatureAllowed,
	}
	act := "Disable"
	if p.SignatureAllowed {
		act = "Enable"
	}
	simplePreview := oas.ActionSimplePreview{
		Name: "SetSignatureAllowed",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "setSignatureAllowedAction",
				Other: "{{.Action}} wallet signature",
			},
			TemplateData: i18n.Template{"Action": act},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &p.Wallet),
	}
	var action oas.OptSetSignatureAllowedAction
	action.SetTo(setSignatureAllowedAction)
	return action, simplePreview
}

func (h *Handler) convertSubscribe(ctx context.Context, a *bath.SubscribeAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptSubscriptionAction, oas.ActionSimplePreview) {
	price := h.convertPrice(ctx, a.Price)
	subscribeAction := oas.SubscriptionAction{
		Price:        price,
		Beneficiary:  convertAccountAddress(a.WithdrawTo, h.addressBook),
		Subscriber:   convertAccountAddress(a.Subscriber, h.addressBook),
		Admin:        convertAccountAddress(a.Admin, h.addressBook),
		Subscription: a.Subscription.ToRaw(),
		Initial:      a.First,
	}
	subscribeAction.Amount.SetTo(a.Price.Amount.Int64()) // for backward compatibility

	value := i18n.FormatTokens(a.Price.Amount, int32(price.Decimals), price.TokenName)

	simplePreview := oas.ActionSimplePreview{
		Name: "Subscription Charge",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "subscriptionAction",
				Other: "Paying {{.Value}} for subscription",
			},
			TemplateData: i18n.Template{"Value": value},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &a.Admin, &a.Subscriber, &a.WithdrawTo),
		Value:    oas.NewOptString(value),
	}
	if a.Price.Amount.Cmp(big.NewInt(0)) == 0 {
		simplePreview.Name = "Subscribed"
		simplePreview.Description = i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "trialSubscriptionAction",
				Other: "Subscription initiated with a delayed payment",
			},
		})
	}
	var action oas.OptSubscriptionAction
	action.SetTo(subscribeAction)
	return action, simplePreview
}

func (h *Handler) convertUnsubscribe(ctx context.Context, a *bath.UnSubscribeAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptUnSubscriptionAction, oas.ActionSimplePreview) {
	simplePreview := oas.ActionSimplePreview{
		Name: "Unsubscribed",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "unsubscribeAction",
				Other: "Subscription deactivated",
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &a.Admin, &a.Subscriber, &a.WithdrawTo),
	}
	var action oas.OptUnSubscriptionAction
	action.SetTo(oas.UnSubscriptionAction{
		Beneficiary:  convertAccountAddress(a.WithdrawTo, h.addressBook),
		Subscriber:   convertAccountAddress(a.Subscriber, h.addressBook),
		Admin:        convertAccountAddress(a.Admin, h.addressBook),
		Subscription: a.Subscription.ToRaw(),
	})
	return action, simplePreview
}

func (h *Handler) convertGasRelayAction(t *bath.GasRelayAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptGasRelayAction, oas.ActionSimplePreview) {
	var action oas.OptGasRelayAction
	action.SetTo(oas.GasRelayAction{
		Amount:  t.Amount,
		Target:  convertAccountAddress(t.Target, h.addressBook),
		Relayer: convertAccountAddress(t.Relayer, h.addressBook),
	})
	simplePreview := oas.ActionSimplePreview{
		Name: "Gas Relay",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "gasRelayAction",
				Other: "Relay for gas",
			},
		}),
		Accounts: distinctAccounts(viewer, h.addressBook, &t.Relayer, &t.Target),
		Value:    oas.NewOptString(i18n.FormatTONs(t.Amount)),
	}
	return action, simplePreview
}
