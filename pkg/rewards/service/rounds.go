package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync/atomic"
	"time"

	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/sync/errgroup"

	"github.com/tonkeeper/opentonapi/pkg/rewards/model"
)

// roundInfo holds intermediate data for a single validation round.
type roundInfo struct {
	electionID     int64
	startUtime     uint32
	endUtime       uint32
	startBlock     uint32
	endBlock       uint32
	finished       bool
	fetchErr       string
	prevElectionID *int64
	nextElectionID *int64
}

// roundFetchErr returns a short, user-friendly error message for a round fetch failure.
func roundFetchErr(err error) string {
	if isPermanentError(err) {
		return "state already gc'd"
	}
	return err.Error()
}

func getAnchorExt(ctx context.Context, client LiteClient, block_seqno *uint32, election_id *int64, unixtime *uint32) (*ton.BlockIDExt, error) {
	var anchorExt ton.BlockIDExt
	switch {
	case block_seqno != nil:
		ext, _, err := lookupMasterchainBlock(ctx, client, *block_seqno)
		if err != nil {
			return nil, fmt.Errorf("lookupMasterchainBlock(%d): %w", *block_seqno, err)
		}
		anchorExt = ext

	case election_id != nil:
		ext, err := lookupMasterchainBlockByUtime(ctx, client, uint32(*election_id))
		if err != nil {
			return nil, fmt.Errorf("lookupMasterchainBlockByUtime(election_id=%d): %w", *election_id, err)
		}
		anchorExt = ext

	case unixtime != nil:
		ext, err := lookupMasterchainBlockByUtime(ctx, client, *unixtime)
		if err != nil {
			return nil, fmt.Errorf("lookupMasterchainBlockByUtime(unixtime=%d): %w", *unixtime, err)
		}
		anchorExt = ext
	}
	return &anchorExt, nil
}

// fetchPrevElectionIDForBlock returns the election_id of the round containing (startBlock - 1).
func fetchPrevElectionIDForBlock(ctx context.Context, client LiteClient, startBlock uint32) (*int64, error) {
	if startBlock <= 1 {
		return nil, fmt.Errorf("startBlock is less than 1")
	}
	currentElectionID, err := getElectionIDForBlock(ctx, client, startBlock)
	if err != nil {
		return nil, err
	}
	if currentElectionID == nil {
		return nil, fmt.Errorf("currentElectionID is nil")
	}
	maxIterations := 1000
	for i := 0; i < maxIterations; i++ {
		startBlock = startBlock - 1
		prevElectionID, err := getElectionIDForBlock(ctx, client, startBlock)
		if err != nil {
			return nil, err
		}
		if prevElectionID == nil {
			return nil, fmt.Errorf("prevElectionID is nil")
		}
		if *prevElectionID == *currentElectionID {
			continue
		}

		return prevElectionID, nil
	}
	return nil, fmt.Errorf("prevElectionID not found")
}

func getElectionIDForBlock(ctx context.Context, client LiteClient, block uint32) (*int64, error) {
	ext, _, err := lookupMasterchainBlock(ctx, client, block)
	if err != nil {
		return nil, err
	}
	return lookupElectionIDForBlock(ctx, client, ext)
}

// lookupElectionIDForBlock reads the election ID from config param 34 at the
// given block. Returns nil on any error.
func lookupElectionIDForBlock(ctx context.Context, client LiteClient, ext ton.BlockIDExt) (*int64, error) {
	since, _, err := getConfigParam34(ctx, client, ext)
	if err != nil {
		return nil, err
	}
	if since == 0 {
		return nil, fmt.Errorf("config param 34 is empty at block %d", ext.Seqno)
	}
	p := int64(since)
	return &p, nil
}

const maxBoundarySearch = 20

// electionBoundary describes two adjacent blocks where the election ID changes.
type electionBoundary struct {
	BeforeBlock      ton.BlockIDExt // last block of the earlier election
	BeforeElectionID int64
	AfterBlock       ton.BlockIDExt // first block of the later election
	AfterElectionID  int64
}

// findElectionBoundary locates where the election ID changes near approxUtime.
// It alternates one step forward, one step backward to find two adjacent blocks
// with different election IDs, minimizing RPC calls when the boundary is close.
func findElectionBoundary(ctx context.Context, client LiteClient, approxUtime uint32) (electionBoundary, error) {
	ext, err := lookupMasterchainBlockByUtime(ctx, client, approxUtime)
	if err != nil {
		return electionBoundary{}, err
	}

	eid, err := lookupElectionIDForBlock(ctx, client, ext)
	if err != nil {
		return electionBoundary{}, err
	}
	if eid == nil {
		return electionBoundary{}, fmt.Errorf("cannot read election ID at block %d", ext.Seqno)
	}

	s0 := ext.Seqno - 2
	s1 := ext.Seqno - 1
	s2 := ext.Seqno
	s3 := ext.Seqno + 1
	s4 := ext.Seqno + 2

	lookBlock := func(seqno uint32) {
		ext, _, err := lookupMasterchainBlock(ctx, client, seqno)
		if err != nil {
			return
		}
		lookupElectionIDForBlock(ctx, client, ext)
	}
	go lookBlock(s0)
	go lookBlock(s1)
	go lookBlock(s2)
	go lookBlock(s3)
	go lookBlock(s4)

	// Walk outward alternating forward/backward to find where the election ID changes.
	fwdExt := ext // rightmost block checked with same ID going forward
	bwdExt := ext // leftmost block checked with same ID going backward
	for i := 0; i < maxBoundarySearch; i++ {
		// One step forward.
		nextExt, _, err := lookupMasterchainBlock(ctx, client, fwdExt.Seqno+1)
		if err == nil {
			nextEID, err := lookupElectionIDForBlock(ctx, client, nextExt)
			if err != nil {
				return electionBoundary{}, err
			}
			if nextEID != nil && *nextEID != *eid {
				return electionBoundary{fwdExt, *eid, nextExt, *nextEID}, nil
			}
			fwdExt = nextExt
		}

		// One step backward.
		if bwdExt.Seqno > 0 {
			prevExt, _, err := lookupMasterchainBlock(ctx, client, bwdExt.Seqno-1)
			if err == nil {
				prevEID, err := lookupElectionIDForBlock(ctx, client, prevExt)
				if err != nil {
					return electionBoundary{}, err
				}
				if prevEID != nil && *prevEID != *eid {
					return electionBoundary{prevExt, *prevEID, bwdExt, *eid}, nil
				}
				bwdExt = prevExt
			}
		}
	}

	return electionBoundary{}, fmt.Errorf("election boundary not found within %d blocks of utime %d", maxBoundarySearch, approxUtime)
}

// getConfigParam34 reads config param 34 pinned to the given block and returns since/until.
func getConfigParam34(ctx context.Context, client LiteClient, ext ton.BlockIDExt) (since, until uint32, err error) {
	pinned := client.WithBlock(ext)
	params, err := cachedGetConfigParams(ctx, pinned, 0, []uint32{34}, ext.Seqno)
	if err != nil {
		return 0, 0, fmt.Errorf("GetConfigParams: %w", err)
	}
	c, _, err := ton.ConvertBlockchainConfig(params, true)
	if err != nil {
		return 0, 0, fmt.Errorf("ConvertBlockchainConfig: %w", err)
	}
	since, until = getRoundInfo(c)

	if since == 0 || until == 0 {
		return 0, 0, fmt.Errorf("config param 34 is empty at block %d", ext.Seqno)
	}
	return since, until, nil
}

// FetchRoundRewards computes per-validator and per-nominator reward distribution
// for a finished validation round using the elector's bonuses value.
func (s *Service) FetchRoundRewards(ctx context.Context, query model.RoundRewardsQuery, shallow bool) (*model.RoundRewardsOutput, error) {
	client := s.currentClient()
	ctx = WithLoaders(ctx, client)

	anchor, err := getAnchorExt(ctx, client, query.Block, query.ElectionID, query.Unixtime)
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

	// Start boundary: transition from prev round → this round near since.
	startBoundary, err := findElectionBoundary(ctx, client, since)
	if err != nil {
		return nil, fmt.Errorf("findElectionBoundary(since): %w", err)
	}

	// End boundary: transition from this round → next round near until.
	endBoundary, err := findElectionBoundary(ctx, client, until)
	if err != nil {
		return nil, fmt.Errorf("findElectionBoundary(until): %w", err)
	}

	electionID := startBoundary.AfterElectionID

	// Pin to last block of this round for config param 34 and pool data.
	currentPinned := client.WithBlock(endBoundary.BeforeBlock)

	// Pin to first block of next round for past elections (available only after round ends).
	nextRoundPinned := client.WithBlock(endBoundary.AfterBlock)

	// fetchRoundData: config and pools from current round, elections from end_block+1.
	rd, err := fetchRoundData(ctx, currentPinned, nextRoundPinned, endBoundary.BeforeBlock.Seqno)
	if err != nil {
		return nil, err
	}

	// Build validator rows.
	rows, totalTrueStake := buildValidatorRows(rd.conf, rd.pools)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no validators found in config param 34 at block %d", endBoundary.BeforeBlock.Seqno)
	}

	if totalTrueStake.Sign() == 0 {
		return nil, fmt.Errorf("total true stake is zero — no pool data available")
	}

	// Find matching election and extract bonuses + total_stake.
	el := findElection(rd.elections, electionID)
	if el == nil || el.Bonuses == nil {
		return nil, fmt.Errorf("election %d not found in past_elections or bonuses not available", electionID)
	}
	bonuses := el.Bonuses
	electionTotalStake := el.TotalStake

	validatorRewards := computeValidatorRewards(ctx, nextRoundPinned, rows, totalTrueStake, bonuses, shallow)

	out := &model.RoundRewardsOutput{
		ElectionID:   electionID,
		RoundStart:   time.Unix(int64(since), 0).UTC(),
		RoundEnd:     time.Unix(int64(until), 0).UTC(),
		StartBlock:   startBoundary.AfterBlock.Seqno,
		EndBlock:     endBoundary.BeforeBlock.Seqno,
		TotalBonuses: &model.NTon{Int: bonuses},
		TotalStake:   &model.NTon{Int: electionTotalStake},
		Validators:   validatorRewards,
	}
	out.PrevElectionID = &startBoundary.BeforeElectionID //fetchPrevElectionIDForBlock(ctx, client, startBlock)
	nextID := endBoundary.AfterElectionID                //int64(until)
	out.NextElectionID = &nextID
	return out, nil
}

// FetchValidationRounds returns metadata for past and current validation rounds.
// It resolves the anchor round sequentially, then estimates middle blocks for
// remaining rounds and fetches them all in parallel.
func (s *Service) FetchValidationRounds(ctx context.Context, query model.RoundsQuery) (*model.ValidationRoundsOutput, error) {
	client := s.currentClient()
	ctx = WithLoaders(ctx, client)

	// Determine anchor block.
	var anchorExt ton.BlockIDExt
	switch {
	case query.Block != nil:
		fallthrough
	case query.ElectionID != nil:
		anchor, err := getAnchorExt(ctx, client, query.Block, query.ElectionID, query.Unixtime)
		if err != nil || anchor == nil {
			return nil, fmt.Errorf("getAnchorExt error or nil: %w", err)
		}
		anchorExt = *anchor
	default:
		ext, err := retry(func() (ton.BlockIDExt, error) {
			model.CountRPC(ctx)
			res, err := client.GetMasterchainInfo(ctx)
			if err != nil {
				return ton.BlockIDExt{}, err
			}
			return ton.BlockIDExt{
				BlockID:  ton.BlockID{Workchain: int32(res.Last.Workchain), Shard: res.Last.Shard, Seqno: res.Last.Seqno},
				RootHash: ton.Bits256(res.Last.RootHash),
				FileHash: ton.Bits256(res.Last.FileHash),
			}, nil
		})
		if err != nil {
			return nil, fmt.Errorf("GetMasterchainInfo: %w", err)
		}
		anchorExt = ext
	}

	limit := 1

	now := time.Now()
	var rounds []roundInfo
	var walkErr string

	// --- Step 1: Resolve anchor round
	anchorSince, anchorUntil, err := getConfigParam34(ctx, client, anchorExt)

	// start async calls to findElectionBoundary so we can use the results later
	go func() {
		findElectionBoundary(ctx, client, anchorSince)
	}()
	go func() {
		findElectionBoundary(ctx, client, anchorUntil)
	}()

	if err != nil {
		walkErr = fmt.Sprintf("stopped after 0 rounds: %v", err)
		log.Printf("warning: %s", walkErr)
		return buildRoundsOutput(rounds, walkErr), nil
	}
	if anchorSince == 0 {
		return buildRoundsOutput(rounds, walkErr), nil
	}

	startExt, err := lookupMasterchainBlockByUtime(ctx, client, anchorSince)
	if err != nil {
		walkErr = fmt.Sprintf("stopped after 0 rounds: %v", err)
		log.Printf("warning: %s", walkErr)
		return buildRoundsOutput(rounds, walkErr), nil
	}

	anchor := roundInfo{
		electionID: int64(anchorSince),
		startUtime: anchorSince,
		endUtime:   anchorUntil,
		startBlock: startExt.Seqno,
	}

	// Determine if anchor round is finished and compute roundLength.
	var roundLength uint32
	if anchorUntil > 0 && time.Unix(int64(anchorUntil), 0).Before(now) {
		anchor.finished = true
		// endExt, err := lookupMasterchainBlockByUtime(ctx, client, anchorUntil)
		endBoundary, err := findElectionBoundary(ctx, client, anchorUntil)
		if err != nil {
			// Use rough estimate for round length.
			log.Printf("warning: could not resolve anchor end_block: %v", err)
			if anchorExt.Seqno > startExt.Seqno {
				roundLength = anchorExt.Seqno - startExt.Seqno
			}
		} else {
			// end_block is the last block of this round.
			anchor.endBlock = endBoundary.BeforeBlock.Seqno
			roundLength = anchor.endBlock - startExt.Seqno
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
		parallelRounds := make([]roundInfo, remaining)
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
					parallelRounds[idx].fetchErr = roundFetchErr(pinErr)
					return nil
				}

				since, until, confErr := getConfigParam34(ctx, client, pinnedExt)
				if confErr != nil {
					parallelRounds[idx].fetchErr = roundFetchErr(confErr)
					return nil
				}
				if since == 0 {
					parallelRounds[idx].fetchErr = "empty config param 34"
					return nil
				}

				sExt, sErr := lookupMasterchainBlockByUtime(ctx, client, since)
				if sErr != nil {
					parallelRounds[idx].fetchErr = roundFetchErr(sErr)
					return nil
				}

				parallelRounds[idx] = roundInfo{
					electionID: int64(since),
					startUtime: since,
					endUtime:   until,
					startBlock: sExt.Seqno,
					finished:   true,
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
			return buildRoundsOutput(rounds, walkErr), nil
		}

		sort.Slice(parallelRounds[:n], func(i, j int) bool {
			return parallelRounds[i].electionID > parallelRounds[j].electionID
		})

		rounds = append(rounds, parallelRounds[:n]...)
	}

	// Derive end_block for rounds after the anchor.
	for i := 1; i < len(rounds); i++ {
		if rounds[i].fetchErr != "" || rounds[i-1].startBlock == 0 {
			continue
		}
		rounds[i].endBlock = rounds[i-1].startBlock - 1
	}

	// Trim to limit.
	if len(rounds) > limit {
		rounds = rounds[:limit]
	}

	err = populatePrevNextElectionIDs(ctx, client, rounds)
	if err != nil {
		return nil, fmt.Errorf("populatePrevNextElectionIDs: %w", err)
	}
	return buildRoundsOutput(rounds, walkErr), nil
}

func populatePrevNextElectionIDs(ctx context.Context, client LiteClient, rounds []roundInfo) error {
	for i := range rounds {
		boundary, err := findElectionBoundary(ctx, client, rounds[i].startUtime)
		if err != nil {
			return err
		}
		rounds[i].prevElectionID = &boundary.BeforeElectionID

		// TODO Are we sure that end utime is always the next election id?
		if rounds[i].finished && rounds[i].endUtime > 0 {
			n := int64(rounds[i].endUtime)
			rounds[i].nextElectionID = &n
		}
	}
	return nil
}

// buildRoundsOutput constructs the final output from collected rounds.
func buildRoundsOutput(rounds []roundInfo, walkErr string) *model.ValidationRoundsOutput {
	out := &model.ValidationRoundsOutput{
		Rounds: make([]model.ValidationRound, len(rounds)),
		Error:  walkErr,
	}
	for i, c := range rounds {
		vr := model.ValidationRound{
			ElectionID:     c.electionID,
			StartBlock:     c.startBlock,
			EndBlock:       c.endBlock,
			Finished:       c.finished,
			Error:          c.fetchErr,
			PrevElectionID: c.prevElectionID,
			NextElectionID: c.nextElectionID,
		}
		if c.startUtime != 0 {
			vr.Start = time.Unix(int64(c.startUtime), 0).UTC()
		}
		if c.endUtime != 0 {
			vr.End = time.Unix(int64(c.endUtime), 0).UTC()
		}
		out.Rounds[i] = vr
	}
	return out
}
