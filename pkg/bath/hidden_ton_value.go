package bath

import (
	"github.com/tonkeeper/tongo"
)

// HiddenTonValue contains TONs amount that is not included in an action directly.
type HiddenTonValue struct {
	Amount   int64
	Sender   tongo.AccountID
	Receiver tongo.AccountID
}

func getTotalHiddenAmount(account tongo.AccountID, values []HiddenTonValue) int64 {
	var extra int64
	for _, ta := range values {
		if ta.Sender == account {
			extra -= ta.Amount
		}
		if ta.Receiver == account {
			extra += ta.Amount
		}
	}
	return extra
}
