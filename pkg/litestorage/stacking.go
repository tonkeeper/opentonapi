package litestorage

import (
	"context"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetParticipatingInWhalesPools(ctx context.Context, member tongo.AccountID) ([]core.WhalesNominator, error) {
	var result []core.WhalesNominator
	for k := range references.WhalesPools {
		_, value, err := abi.GetMember(ctx, s.client, k, member.ToMsgAddress()) //todo: add get_member to tongo and rewrite to usage it
		if err != nil {
			continue
		}
		m, ok := value.(abi.GetMember_WhalesNominatorResult)
		if !ok {
			continue
		}
		if m.MemberBalance+m.MemberPendingDeposit+m.MemberPendingWithdraw+m.MemberPendingDeposit == 0 {
			continue
		}
		result = append(result, core.WhalesNominator{
			Pool:                  k,
			Member:                member,
			MemberBalance:         m.MemberBalance,
			MemberPendingDeposit:  m.MemberPendingDeposit,
			MemberPendingWithdraw: m.MemberPendingWithdraw,
			MemberWithdraw:        m.MemberWithdraw,
		})
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

func (s *LiteStorage) GetTFPools(ctx context.Context) ([]core.TFPool, error) {
	var result []core.TFPool
	for _, a := range s.knownAccounts["tf_pools"] {
		_, v, err := abi.GetPoolData(ctx, s.client, a)
		if err != nil {
			continue
		}
		poolData, ok := v.(abi.GetPoolData_TfResult)
		if !ok {
			continue
		}
		result = append(result, core.TFPool{
			Address:           a,
			TotalAmount:       poolData.StakeAmountSent,
			MinNominatorStake: poolData.MinNominatorStake,
			ValidatorShare:    poolData.ValidatorRewardShare,
			StakeAt:           poolData.StakeAt,
			Nominators:        int(poolData.NominatorsCount),
			MaxNominators:     int(poolData.MaxNominatorsCount),
		})
	}
	return result, nil
}
