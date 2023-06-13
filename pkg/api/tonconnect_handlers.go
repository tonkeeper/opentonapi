package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/tonconnect"
)

func (h Handler) GetTonConnectPayload(ctx context.Context) (res oas.GetTonConnectPayloadRes, err error) {
	payload, err := h.tonConnect.GeneratePayload()
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	return &oas.GetTonConnectPayloadOK{Payload: payload}, nil
}

func (h Handler) TonConnectProof(ctx context.Context, request oas.TonConnectProofReq) (res oas.TonConnectProofRes, err error) {
	proof := tonconnect.Proof{
		Address: request.Address,
		Proof: tonconnect.ProofData{
			Timestamp: request.Proof.Timestamp,
			Domain:    request.Proof.Domain.Value,
			Signature: request.Proof.Signature,
			Payload:   request.Proof.Payload,
			StateInit: request.Proof.StateInit.Value,
		},
	}
	verified, pubKey, err := h.tonConnect.CheckProof(ctx, &proof)
	if err != nil || !verified {
		return &oas.BadRequest{Error: "failed verify proof"}, nil
	}

	hmacHash := hmac.New(sha256.New, []byte(h.tonConnect.GetSecret()))
	hmacHash.Write(pubKey)
	signature := hmacHash.Sum(nil)
	data := append(pubKey, signature...)
	signedToken := base64.URLEncoding.EncodeToString(data)

	return &oas.TonConnectProofOK{Token: signedToken}, nil
}

func (h Handler) GetAccountInfoByStateInit(ctx context.Context, request oas.GetAccountInfoByStateInitReq) (res oas.GetAccountInfoByStateInitRes, err error) {
	pubKey, err := tonconnect.ParseStateInit(request.StateInit)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	cells, _ := boc.DeserializeBocBase64(request.StateInit)
	cellHash, err := cells[0].Hash()
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	accountID := tongo.AccountID{Workchain: int32(0), Address: tlb.Bits256(cellHash[:])}

	return &oas.AccountInfoByStateInit{PublicKey: hex.EncodeToString(pubKey), Address: accountID.ToRaw()}, nil
}
