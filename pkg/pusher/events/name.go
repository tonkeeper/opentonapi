package events

// Name specifies different types of events that Streaming API sends to subscribers.
// Used for accounting purpose.
type Name string

const (
	PingEvent      Name = "ping"
	AccountTxEvent Name = "account-tx"
	TraceEvent     Name = "trace"
	MempoolEvent   Name = "mempool"
)

func (n Name) String() string {
	return string(n)
}
