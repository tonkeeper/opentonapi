package tonconnect

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/wallet"
)

type TonConnect struct {
	Secret string
}

func NewTonConnect(secret string) *TonConnect {
	return &TonConnect{Secret: secret}
}

const (
	tonProofPrefix   = "ton-proof-item-v2/"
	tonConnectPrefix = "ton-connect"
)

var knownHashes = make(map[string]wallet.Version)

func init() {
	for i := wallet.Version(0); i <= wallet.V4R2; i++ {
		ver := wallet.GetCodeHashByVer(i)
		knownHashes[hex.EncodeToString(ver[:])] = i
	}
}

type MessageInfo struct {
	Timestamp int64  `json:"timestamp"`
	Domain    Domain `json:"domain"`
	Signature string `json:"signature"`
	Payload   string `json:"payload"`
	StateInit string `json:"state_init"`
}

type TonProof struct {
	Address string      `json:"address"`
	Proof   MessageInfo `json:"proof"`
}

type Domain struct {
	LengthBytes uint32 `json:"length_bytes"`
	Value       string `json:"value"`
}

type ParsedMessage struct {
	WorkChain int32
	Address   []byte
	TS        int64
	Domain    Domain
	Signature []byte
	Payload   string
	StateInit string
}

func (c *TonConnect) GetSecret() string {
	return c.Secret
}

func (c *TonConnect) GeneratePayload() (string, error) {
	randomBytes := make([]byte, 8)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	hmacHash := hmac.New(sha256.New, []byte(c.Secret))
	hmacHash.Write(randomBytes)
	signature := hmacHash.Sum(nil)

	data := append(randomBytes, signature...)
	payload := base64.URLEncoding.EncodeToString(data)

	return payload, nil
}

func (c *TonConnect) CheckPayload(data string) bool {
	decodedData, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return false
	}

	randomBytes := decodedData[:8]
	signature := decodedData[8:]

	hmacHash := hmac.New(sha256.New, []byte(c.Secret))
	hmacHash.Write(randomBytes)
	computedSignature := hmacHash.Sum(nil)
	if !hmac.Equal(signature, computedSignature) {
		return false
	}

	return true
}

func (c TonConnect) ConvertTonProofMessage(tp *TonProof) (*ParsedMessage, error) {
	addr := strings.Split(tp.Address, ":")
	if len(addr) != 2 {
		return nil, fmt.Errorf("invalid address param: %v", tp.Address)
	}

	workChain, err := strconv.ParseInt(addr[0], 10, 32)
	if err != nil {
		return nil, err
	}

	walletAddr, err := hex.DecodeString(addr[1])
	if err != nil {
		return nil, err
	}

	sig, err := base64.StdEncoding.DecodeString(tp.Proof.Signature)
	if err != nil {
		return nil, err
	}

	return &ParsedMessage{
		WorkChain: int32(workChain),
		Address:   walletAddr,
		Domain:    tp.Proof.Domain,
		TS:        tp.Proof.Timestamp,
		Signature: sig,
		Payload:   tp.Proof.Payload,
		StateInit: tp.Proof.StateInit,
	}, nil
}

func (c TonConnect) CheckProof(tonProofReq *ParsedMessage, pubKey ed25519.PublicKey) (bool, error) {
	mes, err := CreateMessage(tonProofReq)
	if err != nil {
		return false, err
	}
	return SignatureVerify(pubKey, mes, tonProofReq.Signature), nil
}

func CreateMessage(message *ParsedMessage) ([]byte, error) {
	wc := make([]byte, 4)
	binary.BigEndian.PutUint32(wc, uint32(message.WorkChain))

	ts := make([]byte, 8)
	binary.LittleEndian.PutUint64(ts, uint64(message.TS))

	dl := make([]byte, 4)
	binary.LittleEndian.PutUint32(dl, message.Domain.LengthBytes)

	m := []byte(tonProofPrefix)
	m = append(m, wc...)
	m = append(m, message.Address...)
	m = append(m, dl...)
	m = append(m, []byte(message.Domain.Value)...)
	m = append(m, ts...)
	m = append(m, []byte(message.Payload)...)

	messageHash := sha256.Sum256(m)
	fullMes := []byte{0xff, 0xff}
	fullMes = append(fullMes, []byte(tonConnectPrefix)...)
	fullMes = append(fullMes, messageHash[:]...)

	res := sha256.Sum256(fullMes)
	return res[:], nil
}

func SignatureVerify(pubKey ed25519.PublicKey, message, signature []byte) bool {
	return ed25519.Verify(pubKey, message, signature)
}

func ParseStateInit(stateInit string) ([]byte, error) {
	cells, err := boc.DeserializeBocBase64(stateInit)
	if err != nil || len(cells) != 1 {
		return nil, err
	}

	var state tlb.StateInit
	err = tlb.Unmarshal(cells[0], &state)
	if err != nil {
		return nil, err
	}

	if !state.Data.Exists || !state.Code.Exists {
		return nil, err
	}

	hash, err := state.Code.Value.Value.HashString()
	if err != nil {
		return nil, err
	}

	version, prs := knownHashes[hash]
	if !prs {
		return nil, err
	}

	var pubKey tlb.Bits256
	switch version {
	case wallet.V1R1, wallet.V1R2, wallet.V1R3, wallet.V2R1, wallet.V2R2:
		var data wallet.DataV1V2
		err = tlb.Unmarshal(&state.Data.Value.Value, &data)
		if err != nil {
			return nil, err
		}
		pubKey = data.PublicKey

	case wallet.V3R1, wallet.V3R2, wallet.V4R1, wallet.V4R2:
		var data wallet.DataV3
		err = tlb.Unmarshal(&state.Data.Value.Value, &data)
		if err != nil {
			return nil, err
		}
		pubKey = data.PublicKey
	}

	return pubKey[:], nil
}
