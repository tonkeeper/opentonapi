package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"

	"github.com/golang-jwt/jwt"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/tonconnect"
	"github.com/tonkeeper/tongo"
)

type jwtCustomClaims struct {
	Address string `json:"address"`
	jwt.StandardClaims
}

func (h Handler) GetTonConnectPayload(ctx context.Context) (res oas.GetTonConnectPayloadRes, err error) {
	payload, err := h.tonConnect.GeneratePayload()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	return &oas.GetTonConnectPayloadOK{Payload: payload}, nil
}

func (h Handler) TonConnectProof(ctx context.Context, request oas.OptTonConnectProofReq) (res oas.TonConnectProofRes, err error) {
	verified := h.tonConnect.CheckPayload(request.Value.Proof.GetPayload().Value)
	if !verified {
		return &oas.BadRequest{Error: "failed verify payload"}, nil
	}

	requestProof := request.Value.GetProof()
	stateInit := requestProof.StateInit.Value
	proof := tonconnect.TonProof{
		Address: request.Value.GetAddress(),
		Proof: tonconnect.MessageInfo{
			Timestamp: requestProof.Timestamp.Value,
			Domain: tonconnect.Domain{
				LengthBytes: requestProof.Domain.Value.GetLengthBytes().Value,
				Value:       requestProof.Domain.Value.GetValue().Value,
			},
			Signature: requestProof.Signature.Value,
			Payload:   requestProof.Payload.Value,
			StateInit: stateInit,
		},
	}
	parsed, err := h.tonConnect.ConvertTonProofMessage(&proof)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}

	accountID, err := tongo.ParseAccountID(request.Value.GetAddress())
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}

	pubKey, err := h.storage.GetWalletPubKey(accountID)
	if err != nil {
		if stateInit == "" {
			return &oas.BadRequest{Error: "failed get public key"}, nil
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
