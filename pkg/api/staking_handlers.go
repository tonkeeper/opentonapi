package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"

	"github.com/tonkeeper/tongo"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/api/i18n"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
)

func (h Handler) GetStakingPoolInfo(ctx context.Context, params oas.GetStakingPoolInfoParams) (*oas.GetStakingPoolInfoOK, error) {
	poolID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	if w, prs := references.WhalesPools[poolID]; prs {
		poolConfig, poolStatus, nominators, err := h.storage.GetWhalesPoolInfo(ctx, poolID)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		return &oas.GetStakingPoolInfoOK{
			Implementation: oas.PoolImplementation{
				Name:        references.WhalesPoolImplementationsName,
				Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": poolConfig.MinStake / 1_000_000_000}}),
				URL:         references.WhalesPoolImplementationsURL,
			},
			Pool: convertStakingWhalesPool(poolID, w, poolStatus, poolConfig, h.state.GetAPY(), true, nominators),
		}, nil
	}
	lPool, err := h.storage.GetLiquidPool(ctx, poolID)
	if err == nil {
		config, err := h.storage.GetLastConfig()
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		var cycleStart, cycleEnd uint32
		if c, prs := config.Config.Get(34); prs {
			var set tlb.ValidatorsSet
			err = tlb.Unmarshal(&c.Value, &set)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			cycleEnd = set.Common().UtimeUntil
			cycleStart = set.Common().UtimeSince
		}
		return &oas.GetStakingPoolInfoOK{
			Implementation: oas.PoolImplementation{
				Name:        references.LiquidImplementationsName,
				Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": 100}}),
				URL:         references.LiquidImplementationsUrl,
			},
			Pool: convertLiquidStaking(lPool, h.state.GetAPY(), cycleStart, cycleEnd),
		}, nil
	}
	p, err := h.storage.GetTFPool(ctx, poolID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("pool not found: %v", err.Error()))
	}

	info, _ := h.addressBook.GetTFPoolInfo(p.Address)

	return &oas.GetStakingPoolInfoOK{
		Implementation: oas.PoolImplementation{
			Name:        references.TFPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": p.MinNominatorStake / 1_000_000_000}}),
			URL:         references.TFPoolImplementationsURL,
		},
		Pool: convertStakingTFPool(p, info, h.state.GetAPY()),
	}, nil
}

func (h Handler) GetStakingPools(ctx context.Context, params oas.GetStakingPoolsParams) (*oas.GetStakingPoolsOK, error) {
	var result oas.GetStakingPoolsOK
	tfPools, err := h.storage.GetTFPools(ctx, !params.IncludeUnverified.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var minTF, minWhales int64
	var availableFor *tongo.AccountID
	var participatePools []tongo.AccountID
	if params.AvailableFor.IsSet() {
		a, err := tongo.ParseAccountID(params.AvailableFor.Value)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		availableFor = &a
		pools, err := h.storage.GetParticipatingInTfPools(ctx, a)
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusInternalServerError, err)
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

	liquidPools, err := h.storage.GetLiquidPools(ctx, false) //todo: return !params.IncludeUnverified.Value
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	config, err := h.storage.GetLastConfig()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var cycleStart, cycleEnd uint32
	if c, prs := config.Config.Get(34); prs {
		var set tlb.ValidatorsSet
		err = tlb.Unmarshal(&c.Value, &set)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		cycleEnd = set.Common().UtimeUntil
		cycleStart = set.Common().UtimeSince
	}
	for _, p := range liquidPools {
		result.Pools = append(result.Pools, convertLiquidStaking(p, h.state.GetAPY(), cycleStart, cycleEnd))
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

func (h Handler) GetAccountNominatorsPools(ctx context.Context, params oas.GetAccountNominatorsPoolsParams) (*oas.AccountStaking, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	whalesPools, err := h.storage.GetParticipatingInWhalesPools(ctx, accountID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}
	tfPools, err := h.storage.GetParticipatingInTfPools(ctx, accountID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}
	liquidPools, err := h.storage.GetParticipatingInLiquidPools(ctx, accountID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
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
	for _, w := range liquidPools {
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

func (h Handler) GetStakingPoolHistory(ctx context.Context, params oas.GetStakingPoolHistoryParams) (*oas.GetStakingPoolHistoryOK, error) {
	poolID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	_, err = h.storage.GetLiquidPool(ctx, poolID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	logAddress := tlb.MsgAddress{SumType: "AddrExtern"}
	logAddress.AddrExtern = &struct {
		Len             tlb.Uint9
		ExternalAddress boc.BitString
	}{Len: 256, ExternalAddress: g.Must(boc.BitStringFromFiftHex("0000000000000000000000000000000000000000000000000000000000000003"))}
	logs, err := h.storage.GetLogs(ctx, poolID, &logAddress, 100, 0)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var result oas.GetStakingPoolHistoryOK
	var prevTime uint32
	for i, l := range logs {
		if i == 0 {
			prevTime = l.CreatedAt
			continue
		}
		cells, err := boc.DeserializeBoc(l.Body)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		var round struct {
			RoundID  tlb.Uint32
			Borrowed tlb.Coins
			Returned tlb.Coins
			Profit   tlb.SignedCoins
		}
		err = tlb.Unmarshal(cells[0], &round)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		result.Apy = append(result.Apy, oas.ApyHistory{
			Apy:  float64(round.Profit) / float64(round.Borrowed) / float64(l.CreatedAt-prevTime) * 3600 * 24 * 365 * 100,
			Time: int(l.CreatedAt),
		})
		prevTime = l.CreatedAt
		fmt.Printf("%+v\n", round)
	}
	return &result, nil
}
