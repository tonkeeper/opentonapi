package wallet

import (
	"math/big"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	tongoWallet "github.com/tonkeeper/tongo/wallet"
)

// Risk specifies assets that could be lost
// if a message was taken from a malicious actor.
// It makes sense to understand the risk BEFORE sending a message to the blockchain.
type Risk struct {
	// According to https://docs.ton.org/develop/smart-contracts/messages#message-modes
	TransferAllRemainingBalance bool
	Ton                         uint64
	// Jettons are not normalized and have to be post-processed with respect to Jetton masters' decimals.
	Jettons map[tongo.AccountID]big.Int
	Nfts    []tongo.AccountID
}

func ExtractRisk(version tongoWallet.Version, msg *boc.Cell) (*Risk, error) {
	rawMessages, err := tongoWallet.ExtractRawMessages(version, msg)
	if err != nil {
		return nil, err
	}
	return ExtractRiskFromRawMessages(rawMessages)
}

func ExtractRiskFromRawMessages(rawMessages []tongoWallet.RawMessage) (*Risk, error) {
	risk := Risk{
		TransferAllRemainingBalance: false,
		Jettons:                     map[tongo.AccountID]big.Int{},
	}
	for _, rawMsg := range rawMessages {
		if tongoWallet.IsMessageModeSet(int(rawMsg.Mode), tongoWallet.AttachAllRemainingBalance) {
			risk.TransferAllRemainingBalance = true
		}
		var m abi.MessageRelaxed
		rawMsg.Message.ResetCounters() // to avoid the case when several messages are taken from the same cell
		if err := tlb.Unmarshal(rawMsg.Message, &m); err != nil {
			return nil, err
		}
		var err error
		risk, err = ExtractRiskFromMessage(m, risk, rawMsg.Mode)
		if err != nil {
			return nil, err
		}
	}
	return &risk, nil
}

func ExtractRiskFromMessage(m abi.MessageRelaxed, risk Risk, mode byte) (Risk, error) {
	if tongoWallet.IsMessageModeSet(int(mode), tongoWallet.AttachAllRemainingBalance) {
		risk.TransferAllRemainingBalance = true
	}
	tonValue := uint64(m.MessageInternal.Value.Grams)
	destination, err := ton.AccountIDFromTlb(m.MessageInternal.Dest)
	if err != nil {
		return Risk{}, err
	}
	risk.Ton += tonValue
	msgBody := m.MessageInternal.Body.Value.Value
	switch x := msgBody.(type) {
	case abi.NftTransferMsgBody:
		if destination == nil {
			return risk, err
		}
		// here, destination is an NFT
		risk.Nfts = append(risk.Nfts, *destination)
	case abi.JettonBurnMsgBody:
		if destination == nil {
			return risk, err
		}
		// here, destination is a jetton wallet
		amount := big.Int(x.Amount)
		currentJettons := risk.Jettons[*destination]
		var total big.Int
		risk.Jettons[*destination] = *total.Add(&currentJettons, &amount)
	case abi.JettonTransferMsgBody:
		if destination == nil {
			return risk, err
		}
		// here, destination is a jetton wallet
		amount := big.Int(x.Amount)
		currentJettons := risk.Jettons[*destination]
		var total big.Int
		risk.Jettons[*destination] = *total.Add(&currentJettons, &amount)
	}
	return risk, err
}
