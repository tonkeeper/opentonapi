package api

import (
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

func convertStakingWhalesPool(address tongo.AccountID, w references.WhalesPoolInfo, poolStatus abi.GetStakingStatusResult, poolConfig abi.GetParams_WhalesNominatorResult, apy float64, verified bool, nominators int, stake uint64) oas.PoolInfo {

	return oas.PoolInfo{
		Address:           address.ToRaw(),
		Name:              w.Name + " " + w.Queue,
		TotalAmount:       int64(poolStatus.StakeSent),
		NominatorsStake:   int64(stake),
		ValidatorStake:    int64(poolStatus.StakeSent) - int64(stake),
		Implementation:    oas.PoolImplementationTypeWhales,
		Apy:               apy * float64(10000-poolConfig.PoolFee) / 10000,
		MinStake:          poolConfig.MinStake + poolConfig.DepositFee + poolConfig.ReceiptPrice,
		CycleEnd:          int64(poolStatus.StakeUntil),
		CycleStart:        int64(poolStatus.StakeAt),
		Verified:          verified,
		CurrentNominators: nominators,
		MaxNominators:     30000,
		CycleLength:       oas.NewOptInt64(1 << 17),
	}
}

func convertStakingTFPool(p core.TFPool, info addressbook.TFPoolInfo, apy float64) oas.PoolInfo {
	name := info.Name
	if name == "" {
		name = p.Address.ToHuman(true, false)
	}
	return oas.PoolInfo{
		Address:           p.Address.ToRaw(),
		Name:              name,
		TotalAmount:       p.TotalAmount,
		Implementation:    oas.PoolImplementationTypeTf,
		Apy:               apy * float64(10000-p.ValidatorShare) / 10000,
		MinStake:          p.MinNominatorStake + 1_000_000_000, //this is not in contract. just hardcoded value from documentation
		CycleStart:        int64(p.StakeAt),
		CycleEnd:          int64(p.StakeAt) + 3600*36, //todo: make correct
		Verified:          p.VerifiedSources,
		CurrentNominators: p.Nominators,
		MaxNominators:     p.MaxNominators,
		NominatorsStake:   p.NominatorsStake,
		ValidatorStake:    p.ValidatorStake,
		CycleLength:       oas.NewOptInt64(1 << 17),
	}
}

func convertLiquidStaking(p core.LiquidPool, cycleStart, cycleEnd uint32) oas.PoolInfo {
	name := p.Name
	if name == "" {
		name = p.Address.ToHuman(true, false)
	}
	return oas.PoolInfo{
		Address:            p.Address.ToRaw(),
		Name:               name,
		TotalAmount:        p.TotalAmount,
		Implementation:     oas.PoolImplementationTypeLiquidTF,
		Apy:                p.APY,
		MinStake:           int64(ton.OneTON),
		Verified:           p.VerifiedSources,
		CurrentNominators:  p.TotalStakers,
		MaxNominators:      -1,
		LiquidJettonMaster: oas.NewOptString(p.JettonMaster.ToRaw()),
		CycleStart:         int64(cycleStart),
		CycleEnd:           int64(cycleEnd),
		CycleLength:        oas.NewOptInt64(1 << 16),
	}
}
