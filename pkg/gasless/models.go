package gasless

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

type SignRawParams struct {
	RelayAddress string
	Commission   string
	Messages     []Message
	ProtocolName string
}

type TxSendingResults struct {
	ProtocolName string
}
