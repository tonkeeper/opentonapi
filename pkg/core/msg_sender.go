package core

import (
	"math/big"

	"github.com/tonkeeper/tongo"
)

type ExtInMsgCopy struct {
	MsgBoc     string    // base64 encoded message boc.
	Payload    []byte    // decoded message boc.
	Details    any       // optional details from a request context.
	Accounts   *Accounts // set when the message is emulated.
	SendFailed bool      // default is false, so we are good with backward compatibility.
	Meta       map[string]string
}

type Accounts struct {
	TraceID TraceID
	Trace   *Trace
	Actions *ActionsList
}

type ActionsList struct {
	Actions   []Action
	ValueFlow *AccountsValueFlow
}

type AccountsValueFlow struct {
	Accounts map[tongo.AccountID]*AccountValueFlow
}

type AccountValueFlow struct {
	Ton     int64
	Fees    int64
	Jettons map[tongo.AccountID]big.Int
	NFTs    [2]int // 0 - added, 1 - removed
}
