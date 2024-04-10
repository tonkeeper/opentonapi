package core

import (
	"math"

	"github.com/tonkeeper/tongo"
)

type Nominator struct {
	Pool                  tongo.AccountID
	Member                tongo.AccountID
	MemberBalance         int64
	MemberPendingDeposit  int64
	MemberPendingWithdraw int64
	MemberWithdraw        int64
}

type TFPool struct {
	Address         tongo.AccountID
	ValidatorStake  int64
	NominatorsStake int64
	// TotalAmount = ValidatorStake + NominatorsStake
	TotalAmount       int64
	MinNominatorStake int64
	ValidatorShare    uint32
	StakeAt           uint32
	Nominators        int
	MaxNominators     int
	VerifiedSources   bool
}

type LiquidPool struct {
	Name            string
	Address         tongo.AccountID
	TotalAmount     int64
	VerifiedSources bool
	TotalStakers    int
	JettonMaster    tongo.AccountID
	APY             float64
}

type StakingImplementation string

const (
	StakingImplementationTF       StakingImplementation = "tf"
	StakingImplementationLiquidTF StakingImplementation = "liquidTF"
	StakingImplementationWhales   StakingImplementation = "whales"
)

func CalculateAPY(roundExpected, roundBorrowed int64, governanceFee int32) float64 {
	const secondsPerRound = 1 << 16
	const secondsPerYear = 3600 * 24 * 365
	roundsPerYear := float64(secondsPerYear) / float64(secondsPerRound)
	profitPrevRound := float64(roundExpected-roundBorrowed) * (1 - float64(governanceFee)/float64(1<<24))
	percentPerPrevRound := profitPrevRound / float64(roundBorrowed)
	apy := (math.Pow(1+percentPerPrevRound, roundsPerYear) - 1) * 50 //50 its /2*100 because only alf of rounds are effective
	if math.IsNaN(apy) {
		return 0
	}
	return apy
}
