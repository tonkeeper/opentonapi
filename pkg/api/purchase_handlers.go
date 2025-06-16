package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/toncrypto"
	"net/http"
)

func (h *Handler) GetPurchaseHistory(ctx context.Context, params oas.GetPurchaseHistoryParams) (r *oas.AccountPurchases, _ error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	rawPurchases, err := h.storage.GetAccountInvoicesHistory(ctx, account.ID, params.Limit.Value, optIntToPointer(params.BeforeLt))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	purchases, firstLT, err := h.convertPurchaseHistory(ctx, account.ID, rawPurchases)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountPurchases{Purchases: purchases, NextFrom: firstLT}, nil
}

type encryptionParameters struct {
	OurPrivateKey, ReceiverPrivateKey ed25519.PrivateKey
	ReceiverPubkey                    ed25519.PublicKey
	Salt                              []byte
}

func prepareEncryptionParameters() (*encryptionParameters, error) {
	_, ourPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	receiverPubkey, receiverPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	acc := ton.MustParseAccountID("0:0") // TODO: clarify salt
	salt := []byte(acc.ToHuman(true, false))
	return &encryptionParameters{
		OurPrivateKey:      ourPrivateKey,
		ReceiverPrivateKey: receiverPrivateKey,
		ReceiverPubkey:     receiverPubkey,
		Salt:               salt,
	}, nil
}

func (h *Handler) convertPurchaseHistory(ctx context.Context, account ton.AccountID, rawPayments []core.InvoicePayment) ([]oas.Purchase, int64, error) {
	var res []oas.Purchase
	if len(rawPayments) == 0 {
		return []oas.Purchase{}, 0, nil
	}
	params, err := prepareEncryptionParameters()
	if err != nil {
		return nil, 0, err
	}
	for _, raw := range rawPayments {
		p := oas.Purchase{
			EventID:     raw.TraceID.Hash.Hex(),
			InvoiceID:   raw.InvoiceID.String(),
			Source:      convertAccountAddress(account, h.addressBook),
			Destination: convertAccountAddress(raw.Destination, h.addressBook),
			Lt:          int64(raw.InMsgLt),
			Utime:       raw.Utime,
		}
		p.Amount = h.convertPrice(ctx, raw.Amount)
		meta, err := convertMetadata(raw.Metadata, params)
		if err != nil {
			return nil, 0, err
		}
		p.Metadata = meta
		res = append(res, p)
	}
	return res, int64(rawPayments[len(rawPayments)-1].InMsgLt), nil
}

func convertMetadata(m core.PurchaseMetadata, encryptionParameters *encryptionParameters) (oas.Metadata, error) {
	var err error
	switch m.Type {
	case core.TextMetadataType:
		params := encryptionParameters
		if encryptionParameters == nil {
			params, err = prepareEncryptionParameters()
			if err != nil {
				return oas.Metadata{}, err
			}
		}
		return convertInvoiceOpenMetadata(*params, m.Payload)
	case core.EncryptedBinaryMetadataType:
		return oas.Metadata{
			EncryptedBinary: hex.EncodeToString(m.Payload),
		}, nil
	}
	return oas.Metadata{}, nil
}

func convertInvoiceOpenMetadata(params encryptionParameters, data []byte) (oas.Metadata, error) {
	encMeta, err := toncrypto.Encrypt(params.ReceiverPubkey, params.OurPrivateKey, data, params.Salt)
	if err != nil {
		return oas.Metadata{}, err
	}
	res := oas.Metadata{EncryptedBinary: hex.EncodeToString(encMeta)}
	res.DecryptionKey.SetTo(hex.EncodeToString(params.ReceiverPrivateKey))
	return res, nil
}

func (h *Handler) convertPrice(ctx context.Context, price core.Price) oas.Price {
	switch price.Type {
	case core.CurrencyTON:
		return oas.Price{
			CurrencyType: oas.CurrencyTypeNative,
			Value:        price.Amount.String(),
			Decimals:     9,
			TokenName:    "TON",
			Verification: oas.TrustTypeWhitelist,
			Image:        references.TonSymbol,
		}
	case core.CurrencyExtra:
		meta := references.GetExtraCurrencyMeta(*price.CurrencyID)
		return oas.Price{
			CurrencyType: oas.CurrencyTypeExtraCurrency,
			Value:        price.Amount.String(),
			Decimals:     meta.Decimals,
			TokenName:    meta.Symbol,
			Verification: oas.TrustTypeWhitelist,
			Image:        meta.Image,
		}
	case core.CurrencyJetton:
		meta := h.GetJettonNormalizedMetadata(ctx, *price.Jetton)
		res := oas.Price{
			CurrencyType: oas.CurrencyTypeJetton,
			Value:        price.Amount.String(),
			Decimals:     meta.Decimals,
			TokenName:    meta.Symbol,
			Verification: oas.TrustType(meta.Verification),
			Image:        meta.PreviewImage,
		}
		res.Jetton.SetTo(price.Jetton.ToRaw())
		return res
	case core.CurrencyFiat:
		// TODO: fiat not supported yet
	}
	return oas.Price{}
}
