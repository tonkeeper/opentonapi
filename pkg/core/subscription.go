package core

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
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

type SubscriptionV1 struct {
	AccountID            tongo.AccountID
	WalletAccountID      tongo.AccountID
	BeneficiaryAccountID tongo.AccountID
	Status               tlb.AccountStatus
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
