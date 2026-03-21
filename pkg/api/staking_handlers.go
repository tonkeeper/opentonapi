package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo/ton"

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

func (h *Handler) GetStakingPoolInfo(ctx context.Context, params oas.GetStakingPoolInfoParams) (*oas.GetStakingPoolInfoOK, error) {
	pool, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	if w, prs := references.WhalesPools[pool.ID]; prs {
		poolConfig, poolStatus, nominators, stake, err := h.storage.GetWhalesPoolInfo(ctx, pool.ID)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		return &oas.GetStakingPoolInfoOK{
			Implementation: oas.PoolImplementation{
				Name:        references.WhalesPoolImplementationsName,
				Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": poolConfig.MinStake / 1_000_000_000}}),
				URL:         references.WhalesPoolImplementationsURL,
			},
			Pool: convertStakingWhalesPool(pool.ID, w, poolStatus, poolConfig, h.state.GetAPY(), true, nominators, stake),
		}, nil
	}
	lPool, err := h.storage.GetLiquidPool(ctx, pool.ID)
	if err == nil {
		info, _ := h.addressBook.GetAddressInfoByAddress(lPool.Address)
		lPool.Name = info.Name
		config, err := h.storage.GetLastConfig(ctx)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		var cycleStart, cycleEnd uint32
		if param34 := config.ConfigParam34; param34 != nil {
			switch param34.CurValidators.SumType {
			case "Validators":
				cycleEnd = param34.CurValidators.Validators.UtimeUntil + 65536/2 + 600 //magic fron @rulon
				cycleStart = param34.CurValidators.Validators.UtimeSince
			case "ValidatorsExt":
				cycleEnd = param34.CurValidators.ValidatorsExt.UtimeUntil + 65536/2 + 600 //magic fron @rulon
				cycleStart = param34.CurValidators.ValidatorsExt.UtimeSince
			}
		}
		return &oas.GetStakingPoolInfoOK{
			Implementation: oas.PoolImplementation{
				Name:        references.TonstakersImplementationsName,
				Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": 1}}),
				URL:         references.LiquidImplementationsUrl,
				Socials:     references.TonstakersSocialLinks,
			},
			Pool: convertLiquidStaking(lPool, cycleStart, cycleEnd),
		}, nil
	}
	p, err := h.storage.GetTFPool(ctx, pool.ID)
	if err != nil {
		return nil, toError(http.StatusNotFound, fmt.Errorf("pool not found: %v", err.Error()))
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

func (h *Handler) GetStakingPools(ctx context.Context, params oas.GetStakingPoolsParams) (*oas.GetStakingPoolsOK, error) {
	var result oas.GetStakingPoolsOK
	var availableFor *tongo.AccountID
	var participatePools []tongo.AccountID
	if params.AvailableFor.IsSet() {
		account, err := tongo.ParseAddress(params.AvailableFor.Value)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		availableFor = &account.ID
		pools, err := h.storage.GetParticipatingInTfPools(ctx, account.ID)
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusInternalServerError, err)
		}
		for _, p := range pools {
			participatePools = append(participatePools, p.Pool)
		}
	}
	tfPools, err := h.storage.GetTFPools(ctx, !params.IncludeUnverified.Value, availableFor)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var minTF, minWhales int64
	for _, p := range tfPools {
		info, _ := h.addressBook.GetTFPoolInfo(p.Address)
		pool := convertStakingTFPool(p, info, h.state.GetAPY())
		if minTF == 0 || pool.MinStake < minTF {
			minTF = pool.MinStake
		}
		result.Pools = append(result.Pools, pool)
	}
	var participateInWhalePools []core.Nominator
	if availableFor != nil {
		participateInWhalePools, err = h.storage.GetParticipatingInWhalesPools(ctx, *availableFor)
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusInternalServerError, err)
		}
	}
	for k, w := range references.WhalesPools {
		if availableFor != nil && !w.AvailableFor(*availableFor) && !slices.ContainsFunc(participateInWhalePools, func(n core.Nominator) bool {
			return n.Pool == k
		}) { //hide pools which are not available for nominator
			continue
		}
		poolConfig, poolStatus, nominatorsCount, stake, err := h.storage.GetWhalesPoolInfo(ctx, k)
		if err != nil {
			continue
		}
		pool := convertStakingWhalesPool(k, w, poolStatus, poolConfig, h.state.GetAPY(), true, nominatorsCount, stake)
		if minWhales == 0 || pool.MinStake < minWhales {
			minWhales = pool.MinStake
		}
		result.Pools = append(result.Pools, pool)
	}
	liquidPools, err := h.storage.GetLiquidPools(ctx, !params.IncludeUnverified.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	config, err := h.storage.GetLastConfig(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var cycleStart, cycleEnd uint32
	if param34 := config.ConfigParam34; param34 != nil {
		switch param34.CurValidators.SumType {
		case "Validators":
			cycleEnd = param34.CurValidators.Validators.UtimeUntil + 65536/2 + 600 //magic fron @rulon
			cycleStart = param34.CurValidators.Validators.UtimeSince
		case "ValidatorsExt":
			cycleEnd = param34.CurValidators.ValidatorsExt.UtimeUntil + 65536/2 + 600 //magic fron @rulon
			cycleStart = param34.CurValidators.ValidatorsExt.UtimeSince
		}
	}
	for _, p := range liquidPools {
		info, _ := h.addressBook.GetAddressInfoByAddress(p.Address)
		p.Name = info.Name
		result.Pools = append(result.Pools, convertLiquidStaking(p, cycleStart, cycleEnd))
	}
	slices.SortFunc(result.Pools, func(a, b oas.PoolInfo) int {
		if a.Apy == b.Apy {
			return 0
		}
		if a.Apy < b.Apy {
			return -1
		}
		return 1
	})
	result.SetImplementations(map[string]oas.PoolImplementation{
		string(oas.PoolImplementationTypeWhales): {
			Name: references.WhalesPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{DefaultMessage: &i18n.M{
				ID:    "poolImplementationDescription",
				Other: "Minimum deposit {{.Deposit}} TON",
			}, TemplateData: i18n.Template{"Deposit": minWhales / 1_000_000_000}}),
			URL: references.WhalesPoolImplementationsURL,
		},
		string(oas.PoolImplementationTypeTf): {
			Name:        references.TFPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": minTF / 1_000_000_000}}),
			URL:         references.TFPoolImplementationsURL,
		},
		string(oas.PoolImplementationTypeLiquidTF): {
			Name:        references.TonstakersImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": 10}}),
			URL:         references.LiquidImplementationsUrl,
			Socials:     references.TonstakersSocialLinks,
		},
	})
	return &result, nil
}

func (h *Handler) GetAccountNominatorsPools(ctx context.Context, params oas.GetAccountNominatorsPoolsParams) (*oas.AccountStaking, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	whalesPools, err := h.storage.GetParticipatingInWhalesPools(ctx, account.ID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}
	tfPools, err := h.storage.GetParticipatingInTfPools(ctx, account.ID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}
	liquidPools, err := h.storage.GetParticipatingInLiquidPools(ctx, account.ID)
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
		result.Pools = append(result.Pools, convertStaking(w))
	}
	for _, w := range tfPools {
		result.Pools = append(result.Pools, convertStaking(w))
	}
	for _, w := range liquidPools {
		result.Pools = append(result.Pools, convertStaking(w))
	}
	return &result, nil
}
func convertStaking(w core.Nominator) oas.AccountStakingInfo {
	return oas.AccountStakingInfo{
		Pool:            w.Pool.ToRaw(),
		Amount:          w.MemberBalance,
		PendingDeposit:  w.MemberPendingDeposit,
		PendingWithdraw: roundTons(w.MemberPendingWithdraw),
		ReadyWithdraw:   w.MemberWithdraw,
	}
}

func roundTons(amount int64) int64 {
	if amount < int64(ton.OneTON) {
		return amount
	}
	return decimal.New(amount, 0).Round(-7).IntPart()
}

func (h *Handler) GetStakingPoolHistory(ctx context.Context, params oas.GetStakingPoolHistoryParams) (*oas.GetStakingPoolHistoryOK, error) {
	pool, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	_, err = h.storage.GetLiquidPool(ctx, pool.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	logAddress := tlb.MsgAddress{SumType: "AddrExtern"}
	addr := g.Must(boc.BitStringFromFiftHex("0000000000000000000000000000000000000000000000000000000000000003"))
	logAddress.AddrExtern = &addr
	limit := int(params.Limit.Or(100))
	var beforeLT uint64
	if v, ok := params.BeforeLt.Get(); ok {
		beforeLT = uint64(v)
	}
	logs, err := h.storage.GetLogs(ctx, pool.ID, &logAddress, limit, beforeLT)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var result oas.GetStakingPoolHistoryOK
	for _, l := range logs {
		cells, err := boc.DeserializeBoc(l.Body)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		var round struct {
			RoundID      tlb.Uint32
			Borrowed     tlb.Coins
			Returned     tlb.Coins
			Profit       tlb.SignedCoins
			TotalBalance tlb.Coins
			Supply       tlb.Coins
		}
		err = tlb.Unmarshal(cells[0], &round)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		result.Apy = append(result.Apy, oas.ApyHistory{
			Apy:  (math.Pow(float64(round.Returned-round.Borrowed)/float64(round.TotalBalance)+1, 365*24*60*60/float64(65536)) - 1) * 100,
			Time: int(l.CreatedAt),
		})
	}
	return &result, nil
}
