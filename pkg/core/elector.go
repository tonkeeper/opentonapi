package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"time"

	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

var ElectorAccount = ton.MustParseAccountID("-1:3333333333333333333333333333333333333333333333333333333333333333")

type ElectorRoundsIterator struct {
	*ValidatorSetIterator
}

type ValidatorSetIterator struct {
	*liteapi.Client
	ctx context.Context
	err error
}

type ElectorRound struct {
	tlb.ValidatorSet
	ElectorDataPastElection
}

type ElectorDataElect struct {
	ElectAt    uint32
	ElectClose uint32
	MinStake   tlb.Grams
	TotalStake tlb.Grams
	Members    tlb.HashmapE[tlb.Bits256, tlb.Any] `tlb:"256"`
	Failed     bool
	Finished   bool
}

type ElectorDataFrozen struct {
	SrcAddr   tlb.Bits256
	Weight    uint64
	TrueStake tlb.Grams
	Frozen    bool
}

type ElectorDataPastElection struct {
	UnfreezeAt uint32
	StakeHeld  uint32
	VsetHash   tlb.Bits256
	FrozenDict tlb.HashmapE[tlb.Bits256, ElectorDataFrozen] `tlb:"256"`
	TotalStake tlb.Grams
	Bonuses    tlb.Grams
	Complaints tlb.HashmapE[tlb.Bits256, tlb.Any] `tlb:"256"`
}

type ElectorDataCredits struct {
	Amount tlb.Grams
}

type ElectorData struct {
	Elect         tlb.Maybe[tlb.Ref[ElectorDataElect]]
	Credits       tlb.HashmapE[tlb.Bits256, ElectorDataCredits]     `tlb:"256"`
	PastElections tlb.HashmapE[tlb.Uint32, ElectorDataPastElection] `tlb:"32"`
	Grams         tlb.Grams
	ActiveID      uint32
	ActiveHash    tlb.Bits256
}

func NewElectorRoundsIterator(ctx context.Context, client *liteapi.Client) ElectorRoundsIterator {
	return ElectorRoundsIterator{
		NewValidatorSetIterator(ctx, client),
	}
}

func NewValidatorSetIterator(ctx context.Context, client *liteapi.Client) *ValidatorSetIterator {
	return &ValidatorSetIterator{ctx: ctx, Client: client}
}

func (cli *ElectorRoundsIterator) Run(yield func(ElectorRound) bool) {
	var err error
	var pastElections []tlb.HashmapItem[tlb.Uint32, ElectorDataPastElection]
	for i, round := range cli.ValidatorSetIterator.Run {
		if len(pastElections) == 0 {
			blockID := ton.BlockIDExt{
				BlockID: ton.BlockID{
					Workchain: -1,
					Shard:     0x8000000000000000,
				},
			}
			// Get elector data
			var state tlb.ShardAccount
			utime := round.ValidatorsExt.UtimeUntil
			blockID, _, err = cli.LookupBlock(cli.ctx, blockID.BlockID, 4 /* Use utime (Unix time) */, nil, &utime)
			if err != nil {
				if i != 0 {
					cli.err = fmt.Errorf("LookupBlock: %v", err)
					return
				}
				state, err = cli.GetAccountState(cli.ctx, ElectorAccount)
			} else {
				state, err = cli.WithBlock(blockID).GetAccountState(cli.ctx, ElectorAccount)
			}
			if err != nil {
				cli.err = fmt.Errorf("GetAccountState: %w", err)
				return
			}
			if state.Account.SumType != "Account" {
				cli.err = errors.New("elector account not active")
				return
			}
			accountState := state.Account.Account.Storage.State
			if accountState.SumType != "AccountActive" {
				cli.err = errors.New("elector account not active")
				return
			}
			if !accountState.AccountActive.StateInit.Data.Exists {
				cli.err = errors.New("elector account has no data")
				return
			}
			// Parse elector data
			var electorData ElectorData
			dataCell := accountState.AccountActive.StateInit.Data.Value.Value
			err = tlb.Unmarshal(&dataCell, &electorData)
			if err != nil {
				cli.err = fmt.Errorf("failed to parse elector data: %w", err)
				return
			}
			pastElections = electorData.PastElections.Items()
			slices.SortFunc(pastElections, func(a, b tlb.HashmapItem[tlb.Uint32, ElectorDataPastElection]) int {
				return cmp.Compare(b.Key, a.Key)
			})
		}
		for len(pastElections) != 0 && tlb.Uint32(round.ValidatorsExt.UtimeSince) < pastElections[0].Key {
			pastElections = pastElections[1:]
		}
		if len(pastElections) == 0 {
			cli.err = fmt.Errorf("unable to find election at %v", round.ValidatorsExt.UtimeSince)
			return
		}
		if tlb.Uint32(round.ValidatorsExt.UtimeSince) != pastElections[0].Key {
			cli.err = fmt.Errorf("unable to find election at %v", round.ValidatorsExt.UtimeSince)
			return
		}
		if !yield(ElectorRound{round, pastElections[0].Value}) {
			return
		}
		pastElections = pastElections[1:]
	}
}

func (it *ValidatorSetIterator) Run(yield func(int, tlb.ValidatorSet) bool) {
	mc, err := it.GetMasterchainInfo(it.ctx)
	if err != nil {
		it.err = fmt.Errorf("GetMasterchainInfo: %v", err)
		return
	}
	var lastUtimeSince uint32
	id := mc.Last.ToBlockIdExt()
	for i := 0; ; {
		params, err := it.WithBlock(id).GetConfigParams(it.ctx, 0, []uint32{32, 34})
		if err != nil {
			it.err = fmt.Errorf("GetConfigParams: %v", err)
			return
		}
		config, _, err := ton.ConvertBlockchainConfig(params, false)
		if err != nil {
			it.err = fmt.Errorf("GetConfigParams: %v", err)
			return
		}
		rounds := [2]tlb.ValidatorSet{
			config.ConfigParam34.CurValidators,
			config.ConfigParam32.PrevValidators,
		}
		j := 0
		if lastUtimeSince != 0 {
			for ; j < len(rounds); j++ {
				if rounds[j].ValidatorsExt.UtimeUntil == lastUtimeSince {
					break
				}
			}
		}
		if j == len(rounds) {
			return // no new validation rounds found
		}
		for ; j < len(rounds); j++ {
			if !yield(i, rounds[j]) {
				return
			}
			i++
		}
		lastUtimeSince = rounds[1].ValidatorsExt.UtimeSince
		blockID := ton.BlockID{
			Workchain: -1,
			Shard:     0x8000000000000000,
		}
		utime := lastUtimeSince - 1
		id, _, err = it.LookupBlock(it.ctx, blockID, 4 /* Use utime (Unix time) */, nil, &utime)
		if err != nil {
			it.err = fmt.Errorf("LookupBlock: %w", err)
			return
		}
	}
}

func (it *ValidatorSetIterator) Err() error {
	return it.err
}

func (r ElectorRound) ID() uint32 {
	return r.ValidatorsExt.UtimeSince
}

func (r ElectorRound) StartTime() time.Time {
	return time.Unix(int64(r.ValidatorsExt.UtimeSince), 0)
}

func (r ElectorRound) EndTime() time.Time {
	return time.Unix(int64(r.ValidatorsExt.UtimeUntil), 0)
}

func (r ElectorRound) Duration() time.Duration {
	return r.EndTime().Sub(r.StartTime())
}

func (r ElectorRound) APYOrDefault(v float64) float64 {
	if r.Bonuses == 0 {
		log.Println("Zero elector round bonus:", r.StartTime())
		return v
	}
	if r.TotalStake == 0 {
		log.Println("Zero elector round stake:", r.StartTime())
		return v
	}
	roundDuration := r.Duration().Seconds()
	if roundDuration == 0 {
		log.Println("Zero elector round duration:", r.StartTime())
		return v
	}
	const secondsInYear = 365 * 24 * 60 * 60
	yieldPerRound := float64(r.Bonuses) / float64(r.TotalStake)
	roundsPerYear := secondsInYear / roundDuration
	return math.Pow(1+yieldPerRound, roundsPerYear/2) - 1
}

func (r ElectorRound) ProjectedAPY() float64 {
	if r.Bonuses == 0 {
		log.Println("Zero elector round bonus:", r.StartTime())
		return 0
	}
	if r.TotalStake == 0 {
		log.Println("Zero elector round stake:", r.StartTime())
		return 0
	}
	roundDuration := r.Duration().Seconds()
	if roundDuration == 0 {
		log.Println("Zero elector round duration:", r.StartTime())
		return 0
	}
	const secondsInYear = 365 * 24 * 60 * 60
	yieldPerRound := r.ProjectedBonuses() / float64(r.TotalStake)
	roundsPerYear := secondsInYear / roundDuration
	return math.Pow(1+yieldPerRound, roundsPerYear/2) - 1
}

func (r ElectorRound) ProjectedBonuses() float64 {
	timeSinceStart := time.Since(r.StartTime()).Seconds()
	if timeSinceStart == 0 {
		return 0
	}
	return float64(r.Bonuses) * r.Duration().Seconds() / timeSinceStart
}
