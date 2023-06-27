package litestorage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tonkeeper/tongo/tlb"
	"math/big"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetWhalesPoolMemberInfo(ctx context.Context, pool, member tongo.AccountID) (core.Nominator, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_whales_pool_member_info").Observe(v)
	}))
	defer timer.ObserveDuration()
	_, value, err := abi.GetMember(ctx, s.client, pool, member.ToMsgAddress())
	if err != nil {
		return core.Nominator{}, err
	}
	m, ok := value.(abi.GetMember_WhalesNominatorResult)
	if !ok {
		return core.Nominator{}, fmt.Errorf("invalid result")
	}
	if m.MemberBalance+m.MemberWithdraw+m.MemberPendingWithdraw+m.MemberPendingDeposit == 0 {
		return core.Nominator{}, fmt.Errorf("not pool member")
	}
	return core.Nominator{
		Pool:                  pool,
		Member:                member,
		MemberBalance:         m.MemberBalance,
		MemberPendingDeposit:  m.MemberPendingDeposit,
		MemberPendingWithdraw: m.MemberPendingWithdraw,
		MemberWithdraw:        m.MemberWithdraw,
	}, nil
}

func (s *LiteStorage) GetParticipatingInWhalesPools(ctx context.Context, member tongo.AccountID) ([]core.Nominator, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_participating_in_whales_pool").Observe(v)
	}))
	defer timer.ObserveDuration()
	var result []core.Nominator
	for k := range references.WhalesPools {
		info, err := s.GetWhalesPoolMemberInfo(ctx, k, member)
		if err != nil {
			continue
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *LiteStorage) GetParticipatingInTfPools(ctx context.Context, member tongo.AccountID) ([]core.Nominator, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_participating_in_tf_pools").Observe(v)
	}))
	defer timer.ObserveDuration()
	var result []core.Nominator
	fmt.Println(len(s.knownAccounts["tf_pools"]))
	for _, a := range s.knownAccounts["tf_pools"] {
		var i big.Int
		i.SetBytes(member.Address[:])
		_, p, err := abi.GetNominatorData(ctx, s.client, a, tlb.Int257(i))
		if err != nil {
			continue
		}
		if data, ok := p.(abi.GetNominatorDataResult); ok {
			nominator := core.Nominator{
				Pool:                 a,
				Member:               member,
				MemberPendingDeposit: int64(data.PendingDepositAmount),
				MemberBalance:        int64(data.Amount),
			}
			if data.WithdrawFound {
				nominator.MemberPendingWithdraw = nominator.MemberBalance
				nominator.MemberBalance = 0
			}
			result = append(result, nominator)
		}
	}
	return result, nil
}

func (s *LiteStorage) GetWhalesPoolInfo(ctx context.Context, id tongo.AccountID) (abi.GetParams_WhalesNominatorResult, abi.GetStakingStatusResult, int, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_whales_pool_info").Observe(v)
	}))
	defer timer.ObserveDuration()
	var params abi.GetParams_WhalesNominatorResult
	var status abi.GetStakingStatusResult
	var ok bool
	method, value, err := abi.GetParams(ctx, s.client, id)
	if err != nil {
		return params, status, 0, err
	}
	params, ok = value.(abi.GetParams_WhalesNominatorResult)
	if !ok {
		return params, status, 0, fmt.Errorf("get_params returns type %v", method)
	}
	method, value, err = abi.GetStakingStatus(ctx, s.client, id)
	if err != nil {
		return params, status, 0, err
	}
	status, ok = value.(abi.GetStakingStatusResult)
	if !ok {
		return params, status, 0, fmt.Errorf("get_staking returns type %v", method)
	}
	method, value, err = abi.GetMembersRaw(ctx, s.client, id)
	nominators, ok := value.(abi.GetMembersRaw_WhalesNominatorResult)
	if !ok {
		return params, status, 0, fmt.Errorf("get_members returns type %v", method)
	}
	return params, status, len(nominators.Members.List.Keys()), nil
}

func (s *LiteStorage) GetTFPool(ctx context.Context, pool tongo.AccountID) (core.TFPool, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_tf_pool").Observe(v)
	}))
	defer timer.ObserveDuration()
	t, v, err := abi.GetPoolData(ctx, s.client, pool)
	if err != nil {
		return core.TFPool{}, err
	}
	poolData, ok := v.(abi.GetPoolData_TfResult)
	if !ok {
		return core.TFPool{}, fmt.Errorf("invali type %v", t)
	}
	state, err := s.client.GetAccountState(ctx, pool)
	if err != nil {
		return core.TFPool{}, err
	}
	code := state.Account.Account.Storage.State.AccountActive.StateInit.Code.Value.Value
	hash, err := code.Hash()
	if err != nil {
		return core.TFPool{}, err
	}
	return core.TFPool{
		Address:           pool,
		TotalAmount:       poolData.StakeAmountSent,
		MinNominatorStake: poolData.MinNominatorStake,
		ValidatorShare:    poolData.ValidatorRewardShare,
		StakeAt:           poolData.StakeAt,
		Nominators:        int(poolData.NominatorsCount),
		MaxNominators:     int(poolData.MaxNominatorsCount),
		VerifiedSources:   bytes.Equal(hash, references.TFPoolCodeHash[:]),
	}, nil
}
func (s *LiteStorage) GetTFPools(ctx context.Context, onlyVerified bool) ([]core.TFPool, error) {
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
func (s *LiteStorage) GetLiquidPool(ctx context.Context, pool tongo.AccountID) (core.LiquidPool, error) {
	_, v, err := abi.GetPoolFullData(ctx, s.client, pool)
	if err != nil {
		return core.LiquidPool{}, err
	}
	p, ok := v.(abi.GetPoolFullDataResult)
	if !ok {
		return core.LiquidPool{}, fmt.Errorf("invalid type")
	}
	state, err := s.client.GetAccountState(ctx, pool)
	if err != nil {
		return core.LiquidPool{}, err
	}
	code := state.Account.Account.Storage.State.AccountActive.StateInit.Code.Value.Value
	hash, err := code.Hash()
	jettonMaster, err := tongo.AccountIDFromTlb(p.JettonMinter)
	if err != nil || jettonMaster == nil {
		return core.LiquidPool{}, fmt.Errorf("invalid pool jetton %v", jettonMaster)
	}
	return core.LiquidPool{
		Address:         pool,
		TotalAmount:     p.TotalBalance,
		VerifiedSources: bytes.Equal(hash, references.TFLiquidPoolCodeHash[:]),
		JettonMaster:    *jettonMaster,
	}, err
}

func (s *LiteStorage) GetLiquidPools(ctx context.Context, onlyVerified bool) ([]core.LiquidPool, error) {
	return nil, nil
}
