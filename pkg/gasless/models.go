package gasless

import (
	"github.com/tonkeeper/tongo/ton"
)

type Config struct {
	SupportedJettons []string
	RelayAddress     string
}

type Message struct {
	Address   string
	Amount    string
	Payload   string
	StateInit string
}

type EstimationParams struct {
	MasterID                     ton.AccountID
	WalletAddress                ton.AccountID
	WalletPublicKey              []byte
	Messages                     []string
	ReturnEmulation              bool
	ThrowErrorIfNotEnoughJettons bool
}

type SignRawParams struct {
	RelayAddress     string
	Commission       string
	Messages         []Message
	ProtocolName     string
	EmulationResults []byte
}

type TxSendingResults struct {
	ProtocolName string
}
