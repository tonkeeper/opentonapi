package core

import (
	"github.com/tonkeeper/tongo"
)

type WhalesNominator struct {
	Pool                  tongo.AccountID
	Member                tongo.AccountID
	MemberBalance         int64
	MemberPendingDeposit  int64
	MemberPendingWithdraw int64
	MemberWithdraw        int64
}

type TFPool struct {
	Address           tongo.AccountID
	TotalAmount       int64
	MinNominatorStake int64
	ValidatorShare    uint32
	StakeAt           uint32
}
