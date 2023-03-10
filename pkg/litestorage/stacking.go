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
		_, value, err := abi.GetMembers(ctx, s.client, k) //todo: add get_member to tongo and rewrite to usage it
		if err != nil {
			continue
		}
		if members, ok := value.(abi.GetMembers_WhalesNominatorResult); ok {
			for _, m := range members.Members {
				memberID, err := tongo.AccountIDFromTlb(m.Address)
				if err != nil {
					continue
				}
				if memberID != nil && member == *memberID {
					result = append(result, core.WhalesNominator{
						Pool:                  k,
						Member:                member,
						MemberBalance:         m.MemberBalance,
						MemberPendingDeposit:  m.MemberPendingDeposit,
						MemberPendingWithdraw: m.MemberPendingWithdraw,
						MemberWithdraw:        m.MemberWithdraw,
					})
				}
			}
		}
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
