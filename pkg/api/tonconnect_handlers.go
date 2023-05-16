package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"

	"github.com/tonkeeper/opentonapi/pkg/references"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/tonconnect"
	"github.com/tonkeeper/tongo"
)

func (h Handler) GetTonConnectPayload(ctx context.Context) (res oas.GetTonConnectPayloadRes, err error) {
	payload, err := h.tonConnect.GeneratePayload()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	return &oas.GetTonConnectPayloadOK{Payload: payload}, nil
}

func (h Handler) TonConnectProof(ctx context.Context, request oas.TonConnectProofReq) (res oas.TonConnectProofRes, err error) {
	verified := h.tonConnect.CheckPayload(request.Proof.Payload)
	if !verified {
		return &oas.BadRequest{Error: "failed verify payload"}, nil
	}
	requestProof := request.Proof

	if requestProof.Domain.Value != references.AppDomain {
		return &oas.BadRequest{Error: "invalid domain for proof"}, nil
	}
	stateInit := requestProof.StateInit.Value
	proof := tonconnect.TonProof{
		Address: request.Address,
		Proof: tonconnect.MessageInfo{
			Timestamp: requestProof.Timestamp,
			Domain:    requestProof.Domain.Value,
			Signature: requestProof.Signature,
			Payload:   requestProof.Payload,
			StateInit: stateInit,
		},
	}
	parsed, err := h.tonConnect.ConvertTonProofMessage(&proof)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}

	accountID, err := tongo.ParseAccountID(request.Address)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}

	pubKey, err := h.storage.GetWalletPubKey(accountID)
	if err != nil {
		if stateInit == "" {
			return &oas.BadRequest{Error: "failed get public key"}, nil
		}
		if ok, err := tonconnect.CompareStateInitWithAddress(accountID, stateInit); err != nil || !ok {
			return &oas.BadRequest{Error: "failed compare state init with address"}, nil
		}
		pubKey, err = tonconnect.ParseStateInit(stateInit)
		if err != nil {
			return &oas.BadRequest{Error: "failed get public key"}, nil
		}
	}

	check, err := h.tonConnect.CheckProof(parsed, pubKey)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if !check {
		return &oas.BadRequest{Error: "failed proof"}, nil
	}

	hmacHash := hmac.New(sha256.New, []byte(h.tonConnect.GetSecret()))
	hmacHash.Write(pubKey)
	signature := hmacHash.Sum(nil)
	data := append(pubKey, signature...)
	signedToken := base64.URLEncoding.EncodeToString(data)

	return &oas.TonConnectProofOK{Token: signedToken}, nil
}
