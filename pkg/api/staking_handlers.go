package api

import (
	"context"
	"errors"

	"github.com/tonkeeper/tongo"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/api/i18n"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
)

func (h Handler) StakingPoolInfo(ctx context.Context, params oas.StakingPoolInfoParams) (oas.StakingPoolInfoRes, error) {
	poolID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if w, prs := references.WhalesPools[poolID]; prs {
		poolConfig, poolStatus, nominatorsCount, err := h.storage.GetWhalesPoolInfo(ctx, poolID)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		return &oas.StakingPoolInfoOK{
			Implementation: oas.PoolImplementation{
				Name:        references.WhalesPoolImplementationsName,
				Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": poolConfig.MinStake / 1_000_000_000}}),
				URL:         references.WhalesPoolImplementationsURL,
			},
			Pool: convertStakingWhalesPool(poolID, w, poolStatus, poolConfig, h.state.GetAPY(), true, nominatorsCount),
		}, nil
	}
	lPool, err := h.storage.GetLiquidPool(ctx, poolID)
	if err == nil {
		return &oas.StakingPoolInfoOK{
			Implementation: oas.PoolImplementation{
				Name:        references.LiquidImplementationsName,
				Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": 100}}),
				URL:         references.LiquidImplementationsUrl,
			},
			Pool: convertLiquidStaking(lPool, h.state.GetAPY()),
		}, nil
	}
	p, err := h.storage.GetTFPool(ctx, poolID)
	if err != nil {
		return &oas.InternalError{Error: "pool not found: " + err.Error()}, nil
	}

	info, _ := h.addressBook.GetTFPoolInfo(p.Address)

	return &oas.StakingPoolInfoOK{
		Implementation: oas.PoolImplementation{
			Name:        references.TFPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": p.MinNominatorStake / 1_000_000_000}}),
			URL:         references.TFPoolImplementationsURL,
		},
		Pool: convertStakingTFPool(p, info, h.state.GetAPY()),
	}, nil
}

func (h Handler) StakingPools(ctx context.Context, params oas.StakingPoolsParams) (r oas.StakingPoolsRes, _ error) {
	var result oas.StakingPoolsOK
	tfPools, err := h.storage.GetTFPools(ctx, !params.IncludeUnverified.Value)
	if err != nil {
		return nil, err
	}
	var minTF, minWhales int64
	var availableFor *tongo.AccountID
	var participatePools []tongo.AccountID
	if params.AvailableFor.IsSet() {
		a, err := tongo.ParseAccountID(params.AvailableFor.Value)
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
		availableFor = &a
		pools, err := h.storage.GetParticipatingInTfPools(ctx, a)
		if err != nil {
			return nil, err
		}
		for _, p := range pools {
			participatePools = append(participatePools, p.Pool)
		}
	}
	for _, p := range tfPools {
		if availableFor != nil && !slices.Contains(participatePools, p.Address) &&
			(p.Nominators >= p.MaxNominators || //hide nominators without slots
				p.ValidatorShare < 4000 || //hide validators which take less than 40%
				p.MinNominatorStake < 10_000_000_000_000) { //hide nominators with unsafe minimal stake
			continue
		}
		info, _ := h.addressBook.GetTFPoolInfo(p.Address)
		pool := convertStakingTFPool(p, info, h.state.GetAPY())
		if minTF == 0 || pool.MinStake < minTF {
			minTF = pool.MinStake
		}
		result.Pools = append(result.Pools, pool)
	}

	for k, w := range references.WhalesPools {
		if availableFor != nil {
			_, err = h.storage.GetWhalesPoolMemberInfo(ctx, k, *availableFor)
			if err != nil && !w.AvailableFor(*availableFor) {
				continue
			}
		}
		poolConfig, poolStatus, nominatorsCount, err := h.storage.GetWhalesPoolInfo(ctx, k)
		if err != nil {
			continue
		}
		pool := convertStakingWhalesPool(k, w, poolStatus, poolConfig, h.state.GetAPY(), true, nominatorsCount)
		if minWhales == 0 || pool.MinStake < minWhales {
			minWhales = pool.MinStake
		}
		result.Pools = append(result.Pools, pool)
	}

	liquidPools, err := h.storage.GetLiquidPools(ctx, !params.IncludeUnverified.Value)
	if err != nil {
		return nil, err
	}
	for _, p := range liquidPools {
		result.Pools = append(result.Pools, convertLiquidStaking(p, h.state.GetAPY()))
	}

	slices.SortFunc(result.Pools, func(a, b oas.PoolInfo) bool {
		return a.Apy > b.Apy
	})
	result.SetImplementations(map[string]oas.PoolImplementation{
		string(oas.PoolInfoImplementationWhales): {
			Name: references.WhalesPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{DefaultMessage: &i18n.M{
				ID:    "poolImplementationDescription",
				Other: "Minimum deposit {{.Deposit}} TON",
			}, TemplateData: map[string]interface{}{"Deposit": minWhales / 1_000_000_000}}),
			URL: references.WhalesPoolImplementationsURL,
		},
		string(oas.PoolInfoImplementationTf): {
			Name:        references.TFPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": minTF / 1_000_000_000}}),
			URL:         references.TFPoolImplementationsURL,
		},
		string(oas.PoolInfoImplementationLiquidTF): {
			Name:        references.LiquidImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": 10}}),
			URL:         references.LiquidImplementationsUrl,
		},
	})

	return &result, nil
}

func (h Handler) PoolsByNominators(ctx context.Context, params oas.PoolsByNominatorsParams) (oas.PoolsByNominatorsRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	whalesPools, err := h.storage.GetParticipatingInWhalesPools(ctx, accountID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return &oas.NotFound{Error: err.Error()}, nil
		}
		return &oas.InternalError{Error: err.Error()}, nil
	}
	tfPools, err := h.storage.GetParticipatingInTfPools(ctx, accountID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return &oas.NotFound{Error: err.Error()}, nil
		}
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var result oas.AccountStaking
	for _, w := range whalesPools {
		if _, ok := references.WhalesPools[w.Pool]; !ok {
			continue //skip unknown pools
		}
		result.Pools = append(result.Pools, oas.AccountStakingInfo{
			Pool:            w.Pool.ToRaw(),
			Amount:          w.MemberBalance,
			PendingDeposit:  w.MemberPendingDeposit,
			PendingWithdraw: w.MemberPendingWithdraw,
			ReadyWithdraw:   w.MemberWithdraw,
		})
	}
	for _, w := range tfPools {
		result.Pools = append(result.Pools, oas.AccountStakingInfo{
			Pool:            w.Pool.ToRaw(),
			Amount:          w.MemberBalance,
			PendingDeposit:  w.MemberPendingDeposit,
			PendingWithdraw: w.MemberPendingWithdraw,
			ReadyWithdraw:   w.MemberWithdraw,
		})
	}
	return &result, nil
}
