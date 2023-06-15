package core

import (
	"github.com/tonkeeper/tongo"
)

type Subscription struct {
	AccountID            tongo.AccountID
	WalletAccountID      tongo.AccountID
	BeneficiaryAccountID tongo.AccountID
	Amount               int64
	Period               int64
	StartTime            int64
	Timeout              int64
	LastPaymentTime      int64
	LastRequestTime      int64
	FailedAttempts       int32
	SubscriptionID       int64
	IndexerLastUpdateLt  int64
}
