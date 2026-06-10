package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"

	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/sync/errgroup"

	"github.com/tonkeeper/opentonapi/pkg/rewards/model"
	"github.com/tonkeeper/opentonapi/pkg/rewards/utils"
)

func msgAddressToHuman(addr tlb.MsgAddress, bounce bool) (string, bool) {
	id, err := ton.AccountIDFromTlb(addr)
	if err != nil || id == nil {
		return "", false
	}
	return id.ToHuman(bounce, false), true
}

// roundData holds data fetched in parallel for a pinned block.
type roundData struct {
	conf      *ton.BlockchainConfig
	pools     map[tlb.Bits256]poolEntry
	elections []RawPastElection
}

// fetchRoundData fetches config param 34, pool addresses, and past elections in parallel.
// roundPinned is used for config and pool data (should be pinned within the round).
// roundSeqno is the seqno of the pinned block (used as dataloader cache key).
// electionsPinned is used for past elections (may need to be pinned after the round ends).
func fetchRoundData(ctx context.Context, roundPinned, electionsPinned LiteClient, roundSeqno uint32) (roundData, error) {
	var rd roundData
	g := new(errgroup.Group)

	// Config param 34 → validator list.
	g.Go(func() error {
		params, err := cachedGetConfigParams(ctx, roundPinned, 0, []uint32{34}, roundSeqno)
		if err != nil {
			return fmt.Errorf("GetConfigParams: %w", err)
		}
		c, _, err := ton.ConvertBlockchainConfig(params, true)
		if err != nil {
			return fmt.Errorf("ConvertBlockchainConfig: %w", err)
		}
		rd.conf = c
		return nil
	})

	// Pool addresses and true stakes.
	g.Go(func() error {
		p, err := getAllPoolAddresses(ctx, roundPinned, electorAddr)
		if err != nil {
			log.Printf("warning: pool addresses: %v", err)
		}
		rd.pools = p
		return nil
	})

	// Past elections → bonuses and total_stake.
	g.Go(func() error {
		parsed, err := fetchRawPastElections(ctx, electionsPinned, electorAddr)
		if err != nil {
			log.Printf("warning: past elections: %v", err)
			return nil
		}
		rd.elections = parsed
		return nil
	})

	if err := g.Wait(); err != nil {
		return rd, err
	}
	return rd, nil
}

// validatorRow holds intermediate per-validator data used by both stats and rewards.
type validatorRow struct {
	descr     tlb.ValidatorDescr
	trueStake *big.Int
	pool      string
	poolAddr  *ton.AccountID
}

// buildValidatorRows extracts validators from config, computes total true stake,
// and returns rows sorted by stake descending.
func buildValidatorRows(conf *ton.BlockchainConfig, pools map[tlb.Bits256]poolEntry) ([]validatorRow, *big.Int) {
	validators := extractValidators(conf)

	totalTrueStake := new(big.Int)
	for _, v := range validators {
		if pe, ok := pools[v.PubKey()]; ok {
			totalTrueStake.Add(totalTrueStake, pe.TrueStake)
		}
	}

	rows := make([]validatorRow, len(validators))
	for i, v := range validators {
		pk := v.PubKey()
		row := validatorRow{descr: v, trueStake: new(big.Int)}
		if pe, ok := pools[pk]; ok {
			row.pool = pe.Addr.ToHuman(true, false)
			addr := pe.Addr
			row.poolAddr = &addr
			row.trueStake.Set(pe.TrueStake)
		}
		rows[i] = row
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].trueStake.Cmp(rows[j].trueStake) > 0 })

	return rows, totalTrueStake
}

// validatorWeight returns the validator's share of the total true stake.
func validatorWeight(trueStake, totalTrueStake *big.Int) float64 {
	if totalTrueStake.Sign() <= 0 {
		return 0
	}
	return utils.InaccurateDivFloat(trueStake, totalTrueStake)
}

// poolAddressInfo holds resolved pool type and addresses.
type poolAddressInfo struct {
	poolType         string
	validatorAddress string
	ownerAddress     string
	pd               *poolData
}

// resolvePoolAddresses fetches pool data and resolves validator/owner addresses.
func resolvePoolAddresses(ctx context.Context, client LiteClient, poolAddr ton.AccountID) poolAddressInfo {
	poolType, pd := fetchPoolData(ctx, client, poolAddr)
	info := poolAddressInfo{poolType: poolType, pd: pd}

	if pd != nil {
		if vAddr, ok := msgAddressToHuman(pd.ValidatorWalletAddress, true); ok {
			info.validatorAddress = vAddr
		} else if pd.ValidatorAddress != (tlb.Bits256{}) {
			vAddr := ton.AccountID{Workchain: -1, Address: [32]byte(pd.ValidatorAddress)}
			info.validatorAddress = vAddr.ToHuman(true, false)
		}
		if ownerAddr, ok := msgAddressToHuman(pd.OwnerAddress, true); ok {
			info.ownerAddress = ownerAddr
		}
	}

	return info
}

// nominatorPoolMeta holds computed metadata for a nominator pool.
type nominatorPoolMeta struct {
	validatorStake       *big.Int
	nominatorsStake      *big.Int
	totalPoolStake       *big.Int
	validatorRewardShare float64
	nominatorsCount      uint32
}

// computeNominatorPoolMeta extracts nominator pool metadata from pool data.
func computeNominatorPoolMeta(pd *poolData) nominatorPoolMeta {
	meta := nominatorPoolMeta{
		validatorRewardShare: float64(pd.RewardShare) / 10000.0,
		nominatorsCount:      pd.NominatorsCount,
	}
	totalPoolStake := new(big.Int)
	if pd.ValidatorAmount != nil {
		meta.validatorStake = pd.ValidatorAmount
		totalPoolStake.Add(totalPoolStake, pd.ValidatorAmount)
	}
	if pd.NominatorsAmount != nil {
		meta.nominatorsStake = pd.NominatorsAmount
		totalPoolStake.Add(totalPoolStake, pd.NominatorsAmount)
	}
	meta.totalPoolStake = totalPoolStake
	return meta
}

// findElection returns the past election matching the given election ID, or nil.
func findElection(elections []RawPastElection, electAt int64) *RawPastElection {
	for i := range elections {
		if elections[i].ElectAt == electAt {
			return &elections[i]
		}
	}
	return nil
}

// computeValidatorRewards computes per-validator rewards, sorts by stake, and
// enriches each validator with pool data and nominator reward splits in parallel.
// rewardPool is the total reward to distribute (bonuses for round rewards, rewardPerBlock for per-block).
func computeValidatorRewards(ctx context.Context, pinned LiteClient, rows []validatorRow, totalTrueStake, rewardPool *big.Int, shallow bool) []model.ValidatorReward {
	type rewardRow struct {
		validatorRow
		reward *big.Int
	}
	rewardRows := make([]rewardRow, len(rows))
	for i, row := range rows {
		var reward *big.Int
		if totalTrueStake.Sign() > 0 {
			reward = utils.MulDiv(rewardPool, row.trueStake, totalTrueStake)
		} else {
			reward = new(big.Int)
		}
		rewardRows[i] = rewardRow{validatorRow: row, reward: reward}
	}
	sort.Slice(rewardRows, func(i, j int) bool { return rewardRows[i].trueStake.Cmp(rewardRows[j].trueStake) > 0 })

	validatorRewards := make([]model.ValidatorReward, len(rewardRows))
	g := new(errgroup.Group)

	for i, row := range rewardRows {
		g.Go(func() error {
			validatorRewards[i] = model.ValidatorReward{
				Rank:           i + 1,
				Pubkey:         fmt.Sprintf("%x", row.descr.PubKey()),
				EffectiveStake: &model.NanoGram{Int: row.trueStake},
				Weight:         validatorWeight(row.trueStake, totalTrueStake),
				Reward:         &model.NanoGram{Int: row.reward},
				Pool:           row.pool,
			}

			if shallow || row.poolAddr == nil {
				return nil
			}

			poolAddr := *row.poolAddr
			info := resolvePoolAddresses(ctx, pinned, poolAddr)
			validatorRewards[i].PoolType = info.poolType
			validatorRewards[i].ValidatorAddress = info.validatorAddress
			validatorRewards[i].OwnerAddress = info.ownerAddress

			// TotalStake from elector: true_stake + credit (leftover balance kept in contract after election)
			credit, err := computeReturnedStake(ctx, pinned, poolAddr)
			if err != nil {
				log.Printf("warning: computeReturnedStake(%s): %v", poolAddr.ToRaw(), err)
				credit = new(big.Int)
			}

			totalStake := new(big.Int).Add(row.trueStake, credit)
			validatorRewards[i].TotalStake = &model.NanoGram{Int: totalStake}

			if info.pd == nil || info.poolType != poolTypeNominatorV10 {
				return nil
			}

			// Nominator Pool: extract metadata and compute per-nominator rewards.
			meta := computeNominatorPoolMeta(info.pd)
			validatorRewards[i].ValidatorStake = &model.NanoGram{Int: meta.validatorStake}
			validatorRewards[i].NominatorsStake = &model.NanoGram{Int: meta.nominatorsStake}
			validatorRewards[i].ValidatorRewardShare = meta.validatorRewardShare
			validatorRewards[i].NominatorsCount = meta.nominatorsCount

			if info.pd.Nominators == nil {
				return nil
			}

			// Reward split following elector-code.fc:
			// validatorSelfReward = (totalValidatorReward * rewardShare) / 10000
			// nominatorsReward = totalValidatorReward - validatorSelfReward
			rewardShare := big.NewInt(int64(info.pd.RewardShare))
			tenThousand := big.NewInt(10000)
			totalValidatorReward := validatorRewards[i].Reward.Int

			validatorSelfReward := utils.MulDiv(totalValidatorReward, rewardShare, tenThousand)
			if validatorSelfReward.Cmp(totalValidatorReward) > 0 {
				validatorSelfReward.Set(totalValidatorReward)
			}

			nominatorsReward := new(big.Int).Sub(totalValidatorReward, validatorSelfReward)
			nominatorsTotalStake := info.pd.NominatorsAmount

			for _, n := range info.pd.Nominators {
				addr := ton.AccountID{Workchain: 0, Address: tlb.Bits256(n.Address)}
				nominatorStake := new(big.Int).SetUint64(n.Amount)
				nominatorReward := utils.MulDiv(nominatorsReward, nominatorStake, nominatorsTotalStake)

				// total stake 5
				// effective stake 3
				// weight 3/5 = 0.6

				// nominator stake 4
				// nominator effective stake = 4 * 0.6 = 2.4
				//                             4 * 3 / 5 = 2.4

				// nominator effective stake = nominator stake * effective stake / total stake

				nominatorEffectiveStake := utils.MulDiv(nominatorStake, row.trueStake, totalStake)

				validatorRewards[i].Nominators = append(validatorRewards[i].Nominators, model.NominatorReward{
					Address:        addr.ToHuman(true, false),
					Weight:         utils.InaccurateDivFloat(nominatorStake, nominatorsTotalStake),
					Reward:         &model.NanoGram{Int: nominatorReward},
					EffectiveStake: &model.NanoGram{Int: nominatorEffectiveStake},
					Stake:          &model.NanoGram{Int: nominatorStake},
				})
			}

			return nil
		})
	}
	_ = g.Wait()

	return validatorRewards
}
