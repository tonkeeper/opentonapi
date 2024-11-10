package api

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	tongoWallet "github.com/tonkeeper/tongo/wallet"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertToRawMessage(message tongoWallet.RawMessage) (oas.DecodedRawMessage, error) {
	msg := message.Message
	msgBoc, err := msg.ToBocString()
	if err != nil {
		return oas.DecodedRawMessage{}, err
	}
	payload := oas.DecodedRawMessage{
		Message: oas.DecodedRawMessageMessage{
			Boc: msgBoc,
		},
		Mode: int(message.Mode),
	}
	msg.ResetCounters()
	var m tlb.Message
	if err := tlb.Unmarshal(msg, &m); err != nil {
		return oas.DecodedRawMessage{}, err
	}
	msgBody := boc.Cell(m.Body.Value)
	if msgBody.BitsAvailableForRead() >= 32 {
		opcode, err := msgBody.ReadUint(32)
		if err != nil {
			return payload, err
		}
		opcodeStr := oas.NewOptString("0x" + hex.EncodeToString(binary.BigEndian.AppendUint32(nil, uint32(opcode))))
		payload.Message.SetOpCode(opcodeStr)
	}
	msgBody.ResetCounters()
	_, opname, value, err := abi.InternalMessageDecoder(&msgBody, nil)
	if err != nil {
		return payload, nil
	}
	if opname != nil {
		payload.Message.SetDecodedOpName(oas.NewOptString(*opname))
	}
	body, err := json.Marshal(value)
	if err != nil {
		return oas.DecodedRawMessage{}, err
	}
	payload.Message.SetDecodedBody(g.ChangeJsonKeys(body, g.CamelToSnake))
	return payload, nil
}

func (h *Handler) DecodeMessage(ctx context.Context, req *oas.DecodeMessageReq) (*oas.DecodedMessage, error) {
	payloadBytes, err := base64.StdEncoding.DecodeString(req.Boc)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	msg, err := liteapi.ConvertSendMessagePayloadToMessage(payloadBytes)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	accountID, err := extractDestinationWallet(*msg)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	account, err := h.storage.GetAccountState(ctx, *accountID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	ver, ok, err := tongoWallet.GetWalletVersion(account, *msg)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	if !ok {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("not a wallet"))
	}
	msgCell := boc.NewCell()
	if err := tlb.Marshal(msgCell, msg); err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	decoded := oas.DecodedMessage{
		Destination:              convertAccountAddress(*accountID, h.addressBook),
		DestinationWalletVersion: ver.ToString(),
	}
	switch ver {
	case tongoWallet.V4R1, tongoWallet.V4R2:
		v4, err := tongoWallet.DecodeMessageV4(msgCell)
		if err != nil {
			return nil, err
		}
		rawMessages := make([]oas.DecodedRawMessage, 0, len(v4.RawMessages))
		for _, msg := range v4.RawMessages {
			rawMsg, err := convertToRawMessage(msg)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			rawMessages = append(rawMessages, rawMsg)
		}
		extIn := oas.DecodedMessageExtInMsgDecoded{
			WalletV4: oas.NewOptDecodedMessageExtInMsgDecodedWalletV4(oas.DecodedMessageExtInMsgDecodedWalletV4{
				SubwalletID: int64(v4.SubWalletId),
				ValidUntil:  int64(v4.ValidUntil),
				Seqno:       int64(v4.Seqno),
				Op:          int32(v4.Op),
				RawMessages: rawMessages,
			}),
		}
		decoded.SetExtInMsgDecoded(oas.NewOptDecodedMessageExtInMsgDecoded(extIn))
	case tongoWallet.V3R1, tongoWallet.V3R2:
		v3, err := tongoWallet.DecodeMessageV3(msgCell)
		if err != nil {
			return nil, err
		}
		rawMessages := make([]oas.DecodedRawMessage, 0, len(v3.RawMessages))
		for _, msg := range v3.RawMessages {
			rawMsg, err := convertToRawMessage(msg)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			rawMessages = append(rawMessages, rawMsg)
		}
		extIn := oas.DecodedMessageExtInMsgDecoded{
			WalletV3: oas.NewOptDecodedMessageExtInMsgDecodedWalletV3(oas.DecodedMessageExtInMsgDecodedWalletV3{
				SubwalletID: int64(v3.SubWalletId),
				ValidUntil:  int64(v3.ValidUntil),
				Seqno:       int64(v3.Seqno),
				RawMessages: rawMessages,
			}),
		}
		decoded.SetExtInMsgDecoded(oas.NewOptDecodedMessageExtInMsgDecoded(extIn))
	case tongoWallet.HighLoadV2R2, tongoWallet.HighLoadV2R1:
		highload, err := tongoWallet.DecodeHighloadV2Message(msgCell)
		if err != nil {
			return nil, err
		}
		rawMessages := make([]oas.DecodedRawMessage, 0, len(highload.RawMessages))
		for _, msg := range highload.RawMessages {
			rawMsg, err := convertToRawMessage(msg)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			rawMessages = append(rawMessages, rawMsg)
		}
		extIn := oas.DecodedMessageExtInMsgDecoded{
			WalletHighloadV2: oas.NewOptDecodedMessageExtInMsgDecodedWalletHighloadV2(oas.DecodedMessageExtInMsgDecodedWalletHighloadV2{
				SubwalletID:    int64(highload.SubWalletId),
				BoundedQueryID: fmt.Sprintf("%d", highload.BoundedQueryID),
				RawMessages:    rawMessages,
			}),
		}
		decoded.SetExtInMsgDecoded(oas.NewOptDecodedMessageExtInMsgDecoded(extIn))
	case tongoWallet.V5R1:
		v5, err := tongoWallet.DecodeMessageV5(msgCell)
		if err != nil {
			return nil, err
		}
		rawMessages := make([]oas.DecodedRawMessage, 0, len(v5.RawMessages()))
		for _, msg := range v5.RawMessages() {
			rawMsg, err := convertToRawMessage(msg)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			rawMessages = append(rawMessages, rawMsg)
		}
		extIn := oas.DecodedMessageExtInMsgDecoded{
			WalletV5: oas.NewOptDecodedMessageExtInMsgDecodedWalletV5(oas.DecodedMessageExtInMsgDecodedWalletV5{
				RawMessages: rawMessages,
				ValidUntil:  int64(v5.SignedExternal.ValidUntil),
			}),
		}
		decoded.SetExtInMsgDecoded(oas.NewOptDecodedMessageExtInMsgDecoded(extIn))
	default:
		return nil, toError(http.StatusBadRequest, fmt.Errorf("wallet version '%v' is not supported", ver))
	}
	return &decoded, nil
}
