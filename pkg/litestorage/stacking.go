package litestorage

import (
	"context"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetWhalesPoolMemberInfo(ctx context.Context, pool, member tongo.AccountID) (core.WhalesNominator, error) {
	_, value, err := abi.GetMember(ctx, s.client, pool, member.ToMsgAddress())
	if err != nil {
		return core.WhalesNominator{}, err
	}
	m, ok := value.(abi.GetMember_WhalesNominatorResult)
	if !ok {
		return core.WhalesNominator{}, fmt.Errorf("invalid result")
	}
	if m.MemberBalance+m.MemberPendingDeposit+m.MemberPendingWithdraw+m.MemberPendingDeposit == 0 {
		return core.WhalesNominator{}, fmt.Errorf("not pool member")
	}
	return core.WhalesNominator{
		Pool:                  pool,
		Member:                member,
		MemberBalance:         m.MemberBalance,
		MemberPendingDeposit:  m.MemberPendingDeposit,
		MemberPendingWithdraw: m.MemberPendingWithdraw,
		MemberWithdraw:        m.MemberWithdraw,
	}, nil
}

func (s *LiteStorage) GetParticipatingInWhalesPools(ctx context.Context, member tongo.AccountID) ([]core.WhalesNominator, error) {
	var result []core.WhalesNominator
	for k := range references.WhalesPools {
		info, err := s.GetWhalesPoolMemberInfo(ctx, k, member)
		if err != nil {
			continue
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *LiteStorage) GetWhalesPoolInfo(ctx context.Context, id tongo.AccountID) (abi.GetParams_WhalesNominatorResult, abi.GetStakingStatusResult, error) {
	var params abi.GetParams_WhalesNominatorResult
	var status abi.GetStakingStatusResult
	var ok bool
	method, value, err := abi.GetParams(ctx, s.client, id)
	if err != nil {
		return params, status, err
	}
	params, ok = value.(abi.GetParams_WhalesNominatorResult)
	if !ok {
		return params, status, fmt.Errorf("get_params returns type %v", method)
	}
	method, value, err = abi.GetStakingStatus(ctx, s.client, id)
	if err != nil {
		return params, status, err
	}
	status, ok = value.(abi.GetStakingStatusResult)
	if !ok {
		return params, status, fmt.Errorf("get_staking returns type %v", method)
	}
	return params, status, nil
}

func (s *LiteStorage) GetTFPool(ctx context.Context, pool tongo.AccountID) (core.TFPool, error) {
	t, v, err := abi.GetPoolData(ctx, s.client, pool)
	if err != nil {
		return core.TFPool{}, err
	}
	poolData, ok := v.(abi.GetPoolData_TfResult)
	if !ok {
		return core.TFPool{}, fmt.Errorf("invali type %", t)
	}
	return core.TFPool{
		Address:           pool,
		TotalAmount:       poolData.StakeAmountSent,
		MinNominatorStake: poolData.MinNominatorStake,
		ValidatorShare:    poolData.ValidatorRewardShare,
		StakeAt:           poolData.StakeAt,
		Nominators:        int(poolData.NominatorsCount),
		MaxNominators:     int(poolData.MaxNominatorsCount),
	}, nil
}
func (s *LiteStorage) GetTFPools(ctx context.Context) ([]core.TFPool, error) {
	var result []core.TFPool
	for _, a := range s.knownAccounts["tf_pools"] {
		p, err := s.GetTFPool(ctx, a)
		if err != nil {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}
