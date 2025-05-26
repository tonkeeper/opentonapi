package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/toncrypto"
	"net/http"
	"strconv"
)

func (h *Handler) GetInvoiceHistory(ctx context.Context, params oas.GetInvoiceHistoryParams) (r *oas.AccountInvoicePayments, _ error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	rawPayments, err := h.storage.GetAccountInvoicesHistory(ctx, account.ID, params.Limit, optIntToPointer(params.BeforeLt))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	payments, firstLT, err := h.convertInvoiceHistory(ctx, account.ID, rawPayments)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountInvoicePayments{Payments: payments, NextFrom: firstLT}, nil
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

func (h *Handler) convertInvoiceHistory(ctx context.Context, account ton.AccountID, rawPayments []core.InvoicePayment) ([]oas.InvoicePayment, int64, error) {
	var (
		res    []oas.InvoicePayment
		params *encryptionParameters
	)
	if len(rawPayments) == 0 {
		return []oas.InvoicePayment{}, 0, nil
	}
	for _, raw := range rawPayments {
		p := oas.InvoicePayment{
			EventID:     raw.TraceID.Hash.Hex(),
			InvoiceID:   raw.InvoiceID.String(),
			Source:      convertAccountAddress(account, h.addressBook),
			Destination: convertAccountAddress(raw.Destination, h.addressBook),
			Lt:          int64(raw.InMsgLt),
			Utime:       raw.Utime,
		}
		price, err := h.convertInvoicePrice(ctx, raw.Amount, raw.Currency)
		if err != nil {
			return nil, 0, err
		}
		p.Amount = price
		switch raw.MetadataType {
		case core.TextInvoiceMetadataType:
			if params == nil {
				params, err = prepareEncryptionParameters()
				if err != nil {
					return nil, 0, err
				}
			}
			meta, err := convertInvoiceOpenMetadata(*params, raw.Metadata)
			if err != nil {
				return nil, 0, err
			} else {
				p.Metadata = meta
			}
		case core.EncryptedBinaryInvoiceMetadataType:
			p.Metadata.EncryptedBinary = hex.EncodeToString(raw.Metadata)
		default:
			p.Metadata.EncryptedBinary = ""
		}
		res = append(res, p)
	}
	return res, int64(rawPayments[len(rawPayments)-1].InMsgLt), nil
}

func convertInvoiceOpenMetadata(params encryptionParameters, data []byte) (oas.InvoiceMetadata, error) {
	encMeta, err := toncrypto.Encrypt(params.ReceiverPubkey, params.OurPrivateKey, data, params.Salt)
	if err != nil {
		return oas.InvoiceMetadata{}, err
	}
	res := oas.InvoiceMetadata{EncryptedBinary: hex.EncodeToString(encMeta)}
	res.DecryptionKey.SetTo(hex.EncodeToString(params.ReceiverPrivateKey))
	return res, nil
}

func (h *Handler) convertInvoicePrice(ctx context.Context, amount decimal.Decimal, currency string) (oas.Price, error) {
	if len(currency) == 0 {
		return oas.Price{
			Value:     amount.String(),
			Decimals:  9,
			TokenName: "TON",
		}, nil
	}
	jetton, err := ton.ParseAccountID(currency)
	if err != nil {
		id, err := strconv.ParseInt(currency, 10, 32)
		if err != nil {
			return oas.Price{}, errors.New("unknown currency type")
		}
		extraMeta := references.GetExtraCurrencyMeta(int32(id))
		return oas.Price{
			Value:     amount.String(),
			Decimals:  extraMeta.Decimals,
			TokenName: extraMeta.Symbol,
		}, nil
	}
	jettonMeta := h.GetJettonNormalizedMetadata(ctx, jetton)
	return oas.Price{
		Value:     amount.String(),
		Decimals:  jettonMeta.Decimals,
		TokenName: jettonMeta.Symbol,
	}, nil
}
