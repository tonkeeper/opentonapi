package api

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/gasless"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func (h *Handler) GaslessConfig(ctx context.Context) (*oas.GaslessConfig, error) {
	if h.gasless == nil {
		return nil, toError(http.StatusNotImplemented, fmt.Errorf("not implemented"))
	}
	config, err := h.gasless.Config(ctx)
	if err != nil {
		h.logger.Warn("failed to get gasless config", zap.Error(err))
		// TODO:
		// return nil, toError(http.StatusInternalServerError, fmt.Errorf("failed to get gasless config"))
		return &oas.GaslessConfig{
			RelayAddress: "0:dfbd5be8497fdc0c9fcbdfc676864840ddf8ad6423d6d5657d9b0e8270d6c8ac",
			GasJettons: []oas.GaslessConfigGasJettonsItem{
				{MasterID: "0:3702c84f115972f3043a9998a772b282fc290948a5eaaa3ca0d1532c56317f08"},
				{MasterID: "0:78cd9bac1ec6d4daf5533ea8e19689083a8899844742313ef4dc2584ce14cea3"},
				{MasterID: "0:589d4ac897006b5aaa7fae5f95c5e481bd34765664df0b831a9d0eb9ee7fc150"},
				{MasterID: "0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"},
				{MasterID: "0:e8976a15a660739c02fabd0a45e75416de9d0f295eda838544b5ec7cfcc78c1c"},
				{MasterID: "0:ae3e6d351e576276e439e7168117fd64696fd6014cb90c77b2f2cbaacd4fcc00"},
				{MasterID: "0:fe72f474373e97032441bdb873f9a6d3ad10bab08e6dbc7befa5e42b695f5400"},
				{MasterID: "0:bdf3fa8098d129b54b4f73b5bac5d1e1fd91eb054169c3916dfc8ccd536d1000"},
				{MasterID: "0:2f956143c461769579baef2e32cc2d7bc18283f40d20bb03e432cd603ac33ffc"},
				{MasterID: "0:afc49cb8786f21c87045b19ede78fc6b46c51048513f8e9a6d44060199c1bf0c"},
				{MasterID: "0:09f2e59dec406ab26a5259a45d7ff23ef11f3e5c7c21de0b0d2a1cbe52b76b3d"},
				{MasterID: "0:78db4c90b19a1b19ccb45580df48a1e91b6410970fa3d5ffed3eed49e3cf08ff"},
				{MasterID: "0:f4bdd480fcd79d47dbaf6e037d1229115feb2e7ac0f119e160ebd5d031abdf2e"},
			},
		}, nil
	}
	o := &oas.GaslessConfig{
		GasJettons:   make([]oas.GaslessConfigGasJettonsItem, 0, len(config.SupportedJettons)),
		RelayAddress: config.RelayAddress,
	}
	for _, jetton := range config.SupportedJettons {
		o.GasJettons = append(o.GasJettons, oas.GaslessConfigGasJettonsItem{MasterID: jetton})
	}
	return o, nil
}

func (h *Handler) GaslessEstimate(ctx context.Context, req *oas.GaslessEstimateReq, params oas.GaslessEstimateParams) (*oas.SignRawParams, error) {
	if h.gasless == nil {
		return nil, toError(http.StatusNotImplemented, fmt.Errorf("not implemented"))
	}
	masterID, err := ton.ParseAccountID(params.MasterID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid master_id"))
	}
	walletAddress, err := ton.ParseAccountID(req.WalletAddress)
	if err != nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid wallet address"))
	}
	publicKey, err := hex.DecodeString(req.WalletPublicKey)
	if err != nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid public key"))
	}
	messages := make([]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, msg.Boc)
	}
	if params.AcceptLanguage.IsSet() {
		meta := metadata.Pairs("accept-language", params.AcceptLanguage.Value)
		ctx = metadata.NewOutgoingContext(context.Background(), meta)
	}
	estimationParams := gasless.EstimationParams{
		MasterID:                     masterID,
		WalletAddress:                walletAddress,
		WalletPublicKey:              publicKey,
		Messages:                     messages,
		ReturnEmulation:              req.ReturnEmulation.Value,
		ThrowErrorIfNotEnoughJettons: req.ThrowErrorIfNotEnoughJettons.Value,
	}
	signParams, err := h.gasless.Estimate(ctx, estimationParams)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	o := &oas.SignRawParams{
		RelayAddress: signParams.RelayAddress,
		Commission:   signParams.Commission,
		From:         walletAddress.ToRaw(),
		ValidUntil:   time.Now().UTC().Add(4 * time.Minute).Unix(),
		ProtocolName: signParams.ProtocolName,
	}
	if len(signParams.EmulationResults) > 0 {
		var msgConsequences oas.MessageConsequences
		if err := json.Unmarshal(signParams.EmulationResults, &msgConsequences); err != nil {
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("failed to unmarshal emulation results"))
		}
		o.Emulation = oas.NewOptMessageConsequences(msgConsequences)
	}
	o.Messages = make([]oas.SignRawMessage, 0, len(signParams.Messages))
	for _, msg := range signParams.Messages {
		message := oas.SignRawMessage{
			Address: msg.Address,
			Amount:  msg.Amount,
		}
		if len(msg.Payload) > 0 {
			message.Payload = oas.NewOptString(msg.Payload)
		}
		if len(msg.StateInit) > 0 {
			message.StateInit = oas.NewOptString(msg.StateInit)
		}
		o.Messages = append(o.Messages, message)
	}
	return o, nil
}

func (h *Handler) GaslessSend(ctx context.Context, req *oas.GaslessSendReq) (*oas.GaslessTx, error) {
	if h.gasless == nil {
		return nil, toError(http.StatusNotImplemented, fmt.Errorf("not implemented"))
	}
	msg, err := decodeMessage(req.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	pubkey, err := hex.DecodeString(req.WalletPublicKey)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	if len(pubkey) != ed25519.PublicKeySize {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid public key"))
	}
	results, err := h.gasless.Send(ctx, pubkey, msg.payload)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.GaslessTx{ProtocolName: results.ProtocolName}, nil
}
