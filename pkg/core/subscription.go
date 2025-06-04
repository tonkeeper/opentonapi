package core

import (
	"github.com/tonkeeper/tongo"
)

type SubscriptionV2 struct {
	AccountID            tongo.AccountID
	WalletAccountID      tongo.AccountID
	BeneficiaryAccountID tongo.AccountID
	SubscriptionID       int64
	Metadata             []byte
	ContractState        int
	PaymentPerPeriod     int64
	Period               int64
	ChargeDate           int64
	GracePeriod          int64
	LastRequestTime      int64
	CallerFee            int64
}
