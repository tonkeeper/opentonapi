package validation

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	mu           sync.RWMutex
	client       *liteapi.Client
	clientInitAt time.Time
}

type ValidatorsResponse = oas.ValidatorsResponse
type BlockInfo = oas.BlockInfo
type RoundInfo = oas.RoundInfo
type ValidationRound = oas.ValidationRound
type ValidationRoundsResponse = oas.ValidationRoundsResponse
type RoundsQuery = oas.GetValidationRoundsParams
type RoundRewardsQuery = oas.GetRoundRewardsParams
type RoundRewardsResponse = oas.RoundRewardsResponse
type ValidatorReward = oas.ValidatorRewardEntry
type NominatorReward = oas.NominatorRewardEntry

func New(client *liteapi.Client) *Service {
	return &Service{
		client:       client,
		clientInitAt: time.Now(),
	}
}

// FetchPerBlockRewards fetches validator statistics for the given seqno (or latest if nil).
func (s *Service) FetchPerBlockRewards(ctx context.Context, seqno *uint32) (*ValidatorsResponse, error) {
	client := s.client

	// Resolve the target block: use provided seqno or fall back to latest.
	var blockIDExt ton.BlockIDExt
	var blockTime time.Time

	// needBlockTime: for seqno=nil we defer LookupBlock to the parallel group.
	var needBlockTime bool

	if seqno != nil {
		var err error
		blockIDExt, blockTime, err = lookupMasterchainBlock(ctx, client, *seqno)
		if err != nil {
			return nil, fmt.Errorf("lookupMasterchainBlock: %w", err)
		}
	} else {
		res, err := client.GetMasterchainInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("GetMasterchainInfo: %w", err)
		}
		blockIDExt = ton.BlockIDExt{
			BlockID:  ton.BlockID{Workchain: int32(res.Last.Workchain), Shard: res.Last.Shard, Seqno: res.Last.Seqno},
			RootHash: ton.Bits256(res.Last.RootHash),
			FileHash: ton.Bits256(res.Last.FileHash),
		}
		needBlockTime = true
	}

	pinned := client.WithBlock(blockIDExt)

	// Fetch independent data in parallel.
	var (
		rd                roundData
		electorBalance    = new(big.Int)
		rewardPerBlock    = new(big.Int)
		previousElections []RawPastElection
	)

	// fetchGroup: parallel fetches for round data, elector balance, and previous block elections.
	fetchGroup := new(errgroup.Group)

	// Block time (only when seqno was nil — deferred from block resolution).
	if needBlockTime {
		fetchGroup.Go(func() error {
			_, btime, err := lookupMasterchainBlock(ctx, client, blockIDExt.Seqno)
			if err == nil {
				blockTime = btime
			}
			return nil
		})
	}

	// Config, pools, and current elections.
	fetchGroup.Go(func() error {
		r, err := fetchRoundData(ctx, pinned, pinned)
		if err != nil {
			return err
		}
		rd = r
		return nil
	})

	// Elector balance.
	fetchGroup.Go(func() error {
		st, err := pinned.GetAccountState(ctx, electorAddr)
		if err != nil {
			return err
		}
		electorBalance.SetUint64(uint64(st.Account.Account.Storage.Balance.Grams))
		return nil
	})

	// Past elections at previous block (for bonus diff calculation).
	fetchGroup.Go(func() error {
		if blockIDExt.Seqno <= 1 {
			return nil
		}
		prevExt, _, err := lookupMasterchainBlock(ctx, client, blockIDExt.Seqno-1)
		if err != nil {
			return nil
		}
		prevPinned := client.WithBlock(prevExt)
		parsed, err := fetchRawPastElections(ctx, prevPinned, electorAddr)
		if err == nil {
			previousElections = parsed
		}
		return nil
	})

	if err := fetchGroup.Wait(); err != nil {
		return nil, err
	}

	log.Printf("block seqno=%d time=%s", blockIDExt.Seqno, blockTime.UTC().Format(time.RFC3339))

	// Validation round timing from config param 34.
	roundSince, roundUntil := getRoundInfo(rd.conf)

	// Per-block reward = bonus diff between current and previous block in elector.
	electionID := int64(roundSince)
	if cur := findElection(rd.elections, electionID); cur != nil && cur.Bonuses != nil {
		if prev := findElection(previousElections, electionID); prev != nil && prev.Bonuses != nil {
			rewardPerBlock.Sub(cur.Bonuses, prev.Bonuses)
		}
	}

	// blockGroup: resolve round start/end block seqnos in parallel.
	var roundStartBlock, roundEndBlock uint32
	blockGroup := new(errgroup.Group)
	if roundSince > 0 {
		blockGroup.Go(func() error {
			ext, err := lookupMasterchainBlockByUtime(ctx, client, roundSince)
			if err == nil {
				roundStartBlock = ext.Seqno
			}
			return nil
		})
	}
	if roundUntil > 0 && time.Unix(int64(roundUntil), 0).Before(time.Now()) {
		blockGroup.Go(func() error {
			ext, err := lookupMasterchainBlockByUtime(ctx, client, roundUntil)
			if err == nil {
				roundEndBlock = ext.Seqno
			}
			return nil
		})
	}
	_ = blockGroup.Wait()

	out := ValidatorsResponse{
		Block: BlockInfo{
			Seqno: blockIDExt.Seqno,
			Utime: oas.NewOptInt64(blockTime.UTC().Unix()),
		},
		ElectionID:     int64(roundSince),
		ElectorBalance: electorBalance.Int64(),
		RewardPerBlock: rewardPerBlock.Int64(),
	}
	if roundStartBlock > 0 {
		out.PrevElectionID = fetchPrevElectionIDForBlock(ctx, client, roundStartBlock)
	}
	if roundUntil > 0 && time.Unix(int64(roundUntil), 0).Before(time.Now()) {
		nextID := int64(roundUntil)
		out.NextElectionID = oas.NewOptInt64(nextID)
	}
	out.ValidationRound = RoundInfo{
		StartUtime: oas.NewOptInt64(int64(roundSince)),
		EndUtime:   oas.NewOptInt64(int64(roundUntil)),
		StartBlock: roundStartBlock,
		EndBlock:   roundEndBlock,
	}

	// Build validator rows.
	rows, totalTrueStake := buildValidatorRows(rd.conf, rd.pools)
	log.Printf("active validators: %d", len(rows))
	out.TotalStake = totalTrueStake.Int64()
	log.Printf("total true stake (active validators): %.2f TON", new(big.Float).Quo(new(big.Float).SetInt(totalTrueStake), big.NewFloat(1e9)))

	out.Validators = computeValidatorRewards(ctx, pinned, rows, totalTrueStake, rewardPerBlock)
	return &out, nil
}

// FetchRoundRewards computes per-validator and per-nominator reward distribution
// for a finished validation round using the elector's bonuses value.
func (s *Service) FetchRoundRewards(ctx context.Context, query RoundRewardsQuery) (*RoundRewardsResponse, error) {
	client := s.client

	anchor, err := getAnchorExt(ctx, client, query.Block, query.ElectionID)
	if err != nil || anchor == nil {
		return nil, fmt.Errorf("getAnchorExt error or nil: %w", err)
	}
	anchorExt := *anchor

	// Resolve round boundaries from config param 34.
	since, until, err := getConfigParam34(ctx, client, anchorExt)
	if err != nil {
		return nil, fmt.Errorf("getConfigParam34: %w", err)
	}
	if since == 0 {
		return nil, fmt.Errorf("config param 34 is empty at block %d", anchorExt.Seqno)
	}

	// Verify the round is finished.
	if time.Unix(int64(until), 0).After(time.Now()) {
		return nil, fmt.Errorf("round %d is not finished yet (ends %s)", since, time.Unix(int64(until), 0).UTC().Format(time.RFC3339))
	}

	// Resolve start_block and end_block.
	startExt, err := lookupMasterchainBlockByUtime(ctx, client, since)
	if err != nil {
		return nil, fmt.Errorf("lookupMasterchainBlockByUtime(since=%d): %w", since, err)
	}
	endExt, err := lookupMasterchainBlockByUtime(ctx, client, until)
	if err != nil {
		return nil, fmt.Errorf("lookupMasterchainBlockByUtime(until=%d): %w", until, err)
	}
	endBlock := endExt.Seqno - 1 // end_block is the last block of this round
	nextRoundFirstBlock := endExt.Seqno

	// Pin to end_block for current round data (config param 34, pools).
	currentRoundExt, _, err := lookupMasterchainBlock(ctx, client, endBlock)
	if err != nil {
		return nil, fmt.Errorf("lookupMasterchainBlock(end=%d): %w", endBlock, err)
	}
	currentPinned := client.WithBlock(currentRoundExt)

	// Pin to end_block + 1 for past elections (available only after round ends).
	nextRoundExt, _, err := lookupMasterchainBlock(ctx, client, nextRoundFirstBlock)
	if err != nil {
		return nil, fmt.Errorf("lookupMasterchainBlock(nextRoundFirstBlock=%d): %w", nextRoundFirstBlock, err)
	}
	nextRoundPinned := client.WithBlock(nextRoundExt)

	// fetchRoundData: config and pools from current round, elections from end_block+1.
	rd, err := fetchRoundData(ctx, currentPinned, nextRoundPinned)
	if err != nil {
		return nil, err
	}

	// Build validator rows.
	rows, totalTrueStake := buildValidatorRows(rd.conf, rd.pools)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no validators found in config param 34 at block %d", currentRoundExt.Seqno)
	}

	if totalTrueStake.Sign() == 0 {
		return nil, fmt.Errorf("total true stake is zero — no pool data available")
	}

	// Find matching election and extract bonuses + total_stake.
	electionID := int64(since)
	el := findElection(rd.elections, electionID)
	if el == nil || el.Bonuses == nil {
		return nil, fmt.Errorf("election %d not found in past_elections or bonuses not available", electionID)
	}
	bonuses := el.Bonuses
	electionTotalStake := el.TotalStake

	validatorRewards := computeValidatorRewards(ctx, nextRoundPinned, rows, totalTrueStake, bonuses)

	out := RoundRewardsResponse{
		ElectionID:   electionID,
		RoundStart:   time.Unix(int64(since), 0).UTC(),
		RoundEnd:     time.Unix(int64(until), 0).UTC(),
		StartBlock:   startExt.Seqno,
		EndBlock:     endBlock,
		TotalBonuses: bonuses.Int64(),
		TotalStake:   electionTotalStake.Int64(),
		Validators:   validatorRewards,
	}
	out.PrevElectionID = fetchPrevElectionIDForBlock(ctx, client, startExt.Seqno)

	out.NextElectionID = oas.NewOptInt64(int64(until))
	return &out, nil
}

// FetchValidationRounds returns metadata for past and current validation rounds.
// It resolves the anchor round sequentially, then estimates middle blocks for
// remaining rounds and fetches them all in parallel.
func (s *Service) FetchValidationRounds(ctx context.Context, query RoundsQuery) ([]ValidationRound, error) {
	client := s.client

	// Determine anchor block.
	var anchorExt ton.BlockIDExt
	switch {
	case query.Block.Set:
		fallthrough
	case query.ElectionID.Set:
		anchor, err := getAnchorExt(ctx, client, query.Block, query.ElectionID)
		if err != nil || anchor == nil {
			return nil, fmt.Errorf("getAnchorExt error or nil: %w", err)
		}
		anchorExt = *anchor
	default:
		res, err := client.GetMasterchainInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("GetMasterchainInfo: %w", err)
		}
		anchorExt = ton.BlockIDExt{
			BlockID:  ton.BlockID{Workchain: int32(res.Last.Workchain), Shard: res.Last.Shard, Seqno: res.Last.Seqno},
			RootHash: ton.Bits256(res.Last.RootHash),
			FileHash: ton.Bits256(res.Last.FileHash),
		}
	}

	limit := 1

	now := time.Now()
	var rounds []ValidationRound
	var walkErr string

	// --- Step 1: Resolve anchor round
	anchorSince, anchorUntil, err := getConfigParam34(ctx, client, anchorExt)
	if err != nil {
		walkErr = fmt.Sprintf("stopped after 0 rounds: %v", err)
		log.Printf("warning: %s", walkErr)
		return rounds, nil
	}
	if anchorSince == 0 {
		return rounds, nil
	}

	startExt, err := lookupMasterchainBlockByUtime(ctx, client, anchorSince)
	if err != nil {
		walkErr = fmt.Sprintf("stopped after 0 rounds: %v", err)
		log.Printf("warning: %s", walkErr)
		return rounds, nil
	}

	anchor := ValidationRound{
		ElectAt:    oas.NewOptInt64(int64(anchorSince)),
		StartUtime: oas.NewOptInt64(int64(anchorSince)),
		EndUtime:   oas.NewOptInt64(int64(anchorUntil)),
		StartBlock: startExt.Seqno,
	}

	// Determine if anchor round is finished and compute roundLength.
	var roundLength uint32
	if anchorUntil > 0 && time.Unix(int64(anchorUntil), 0).Before(now) {
		anchor.Finished = true
		endExt, err := lookupMasterchainBlockByUtime(ctx, client, anchorUntil)
		if err != nil {
			// Use rough estimate for round length.
			log.Printf("warning: could not resolve anchor end_block: %v", err)
			if anchorExt.Seqno > startExt.Seqno {
				roundLength = anchorExt.Seqno - startExt.Seqno
			}
		} else {
			// lookupByUtime returns the first block of the next round;
			// end_block is the last block of this round.
			anchor.EndBlock = oas.NewOptUint32(endExt.Seqno - 1)
			roundLength = endExt.Seqno - startExt.Seqno
		}
	} else {
		// Unfinished round — extrapolate full round length from partial data.
		partialBlocks := anchorExt.Seqno - startExt.Seqno
		elapsed := uint32(now.Unix()) - anchorSince
		fullDuration := anchorUntil - anchorSince
		if partialBlocks > 0 && elapsed > 0 && fullDuration > 0 {
			roundLength = partialBlocks * fullDuration / elapsed
		}
	}

	rounds = append(rounds, anchor)

	// --- Step 2+3: Estimate middle blocks and fan out in parallel ---
	remaining := limit - 1
	if remaining > 0 && roundLength > 0 && startExt.Seqno > 1 {
		parallelRounds := make([]ValidationRound, remaining)
		g := new(errgroup.Group)

		var launched atomic.Int32
		for i := 1; i <= remaining; i++ {
			offset := roundLength/2 + uint32(i-1)*roundLength
			if offset >= startExt.Seqno {
				break
			}
			middleBlock := startExt.Seqno - offset

			g.Go(func() error {
				idx := int(launched.Add(1)) - 1

				pinnedExt, _, pinErr := lookupMasterchainBlock(ctx, client, middleBlock)
				if pinErr != nil {
					return nil
				}

				since, until, confErr := getConfigParam34(ctx, client, pinnedExt)
				if confErr != nil {
					return nil
				}
				if since == 0 {
					return nil
				}

				sExt, sErr := lookupMasterchainBlockByUtime(ctx, client, since)
				if sErr != nil {
					return nil
				}

				parallelRounds[idx] = ValidationRound{
					ElectAt:    oas.NewOptInt64(int64(since)),
					StartUtime: oas.NewOptInt64(int64(since)),
					EndUtime:   oas.NewOptInt64(int64(until)),
					StartBlock: sExt.Seqno,
					Finished:   true,
				}
				return nil
			})
		}

		err = g.Wait()
		n := int(launched.Load())
		if err != nil {
			walkErr = fmt.Sprintf("stopped after %d rounds: %v", n, err)
			log.Printf("warning: %s", walkErr)
			populatePrevNextElectionIDs(ctx, client, rounds)
			return rounds, nil
		}

		sort.Slice(parallelRounds[:n], func(i, j int) bool {
			return parallelRounds[i].ElectAt.Value > parallelRounds[j].ElectAt.Value
		})

		rounds = append(rounds, parallelRounds[:n]...)
	}

	// Derive end_block for rounds after the anchor.
	for i := 1; i < len(rounds); i++ {
		if rounds[i-1].StartBlock == 0 {
			continue
		}
		rounds[i].EndBlock = oas.NewOptUint32(rounds[i-1].StartBlock - 1)
	}

	// Trim to limit.
	if len(rounds) > limit {
		rounds = rounds[:limit]
	}

	populatePrevNextElectionIDs(ctx, client, rounds)
	return rounds, nil
}

func getAnchorExt(ctx context.Context, client *liteapi.Client, block_seqno oas.OptUint32, election_id oas.OptInt64) (*ton.BlockIDExt, error) {
	var anchorExt ton.BlockIDExt
	switch {
	case block_seqno.Set:
		ext, _, err := lookupMasterchainBlock(ctx, client, block_seqno.Value)
		if err != nil {
			return nil, fmt.Errorf("lookupMasterchainBlock(%d): %w", block_seqno.Value, err)
		}
		anchorExt = ext

	case election_id.Set:
		ext, err := lookupMasterchainBlockByUtime(ctx, client, uint32(election_id.Value))
		if err != nil {
			return nil, fmt.Errorf("lookupMasterchainBlockByUtime(election_id=%d): %w", election_id.Value, err)
		}
		anchorExt = ext
	}
	return &anchorExt, nil
}

// fetchPrevElectionIDForBlock returns the election_id of the round containing (startBlock - 1).
func fetchPrevElectionIDForBlock(ctx context.Context, client *liteapi.Client, startBlock uint32) oas.OptInt64 {
	if startBlock <= 1 {
		return oas.OptInt64{}
	}
	prevBlock := startBlock - 1
	pinnedExt, _, err := lookupMasterchainBlock(ctx, client, prevBlock)
	if err != nil {
		return oas.OptInt64{}
	}
	since, _, err := getConfigParam34(ctx, client, pinnedExt)
	if err != nil || since == 0 {
		return oas.OptInt64{}
	}
	return oas.NewOptInt64(int64(since))
}

// getConfigParam34 reads config param 34 pinned to the given block and returns since/until.
func getConfigParam34(ctx context.Context, client *liteapi.Client, ext ton.BlockIDExt) (since, until uint32, err error) {
	pinned := client.WithBlock(ext)
	params, err := pinned.GetConfigParams(ctx, 0, []uint32{34})
	if err != nil {
		return 0, 0, fmt.Errorf("GetConfigParams: %w", err)
	}
	conf, _, err := ton.ConvertBlockchainConfig(params, true)
	if err != nil {
		return 0, 0, fmt.Errorf("ConvertBlockchainConfig: %w", err)
	}
	since, until = getRoundInfo(conf)
	return since, until, nil
}

func populatePrevNextElectionIDs(ctx context.Context, client *liteapi.Client, rounds []ValidationRound) {
	for i := range rounds {
		rounds[i].PrevElectionID = fetchPrevElectionIDForBlock(ctx, client, rounds[i].StartBlock)

		// TODO Are we sure that end utime is always the next election id?
		if rounds[i].Finished && rounds[i].EndUtime.Set {
			rounds[i].NextElectionID = oas.NewOptInt64(rounds[i].EndUtime.Value)
		}
	}
}
