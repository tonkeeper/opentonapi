package api

import (
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func convertStakingWhalesPool(address tongo.AccountID, w references.WhalesPoolInfo, poolStatus abi.GetStakingStatusResult, poolConfig abi.GetParams_WhalesNominatorResult, apy float64, verified bool) oas.PoolInfo {
	return oas.PoolInfo{
		Address:           address.ToRaw(),
		Name:              w.Name + " " + w.Queue,
		TotalAmount:       int64(poolStatus.StakeSent),
		Implementation:    oas.PoolInfoImplementationWhales,
		Apy:               apy * float64(10000-poolConfig.PoolFee) / 10000,
		MinStake:          poolConfig.MinStake + poolConfig.DepositFee + poolConfig.ReceiptPrice,
		CycleEnd:          int64(poolStatus.StakeUntil),
		CycleStart:        int64(poolStatus.StakeAt),
		Verified:          verified,
		CurrentNominators: 0, //todo: add actual values
		MaxNominators:     40000,
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
		Implementation:    oas.PoolInfoImplementationTf,
		Apy:               apy * float64(10000-p.ValidatorShare) / 10000,
		MinStake:          p.MinNominatorStake + 1_000_000_000, //this is not in contract. just hardcoded value from documentation
		CycleStart:        int64(p.StakeAt),
		CycleEnd:          int64(p.StakeAt) + 3600*36, //todo: make correct
		Verified:          p.VerifiedSources,
		CurrentNominators: p.Nominators,
		MaxNominators:     p.MaxNominators,
	}
}
