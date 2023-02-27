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
