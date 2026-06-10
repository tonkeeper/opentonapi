package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/sync/errgroup"

	"github.com/tonkeeper/opentonapi/pkg/rewards/model"
)

// FetchPerBlockRewards fetches validator statistics for the given seqno (or latest if nil).
func (s *Service) FetchPerBlockRewards(ctx context.Context, seqno *uint32, unixtime *uint32, shallow bool) (*model.Output, error) {
	client := s.currentClient()
	ctx = WithLoaders(ctx, client)

	// Resolve the target block: use provided seqno, unixtime, or fall back to latest.
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
	} else if unixtime != nil {
		ext, err := lookupMasterchainBlockByUtime(ctx, client, *unixtime)
		if err != nil {
			return nil, fmt.Errorf("lookupMasterchainBlockByUtime(%d): %w", *unixtime, err)
		}
		blockIDExt = ext
		needBlockTime = true
	} else {
		info, err := retry(func() (ton.BlockIDExt, error) {
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
		blockIDExt = info
		needBlockTime = true
	}

	pinned := client.WithBlock(blockIDExt)

	// Fetch independent data in parallel.
	var (
		rd                roundData
		electorBalance    = new(big.Int)
		rewardPerBlock    = new(big.Int)
		previousElections []RawPastElection
		prevBlockBonuses  = new(big.Int)
		currBlockBonuses  = new(big.Int)
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
		r, err := fetchRoundData(ctx, pinned, pinned, blockIDExt.Seqno)
		if err != nil {
			return err
		}
		rd = r
		return nil
	})

	// Elector balance.
	fetchGroup.Go(func() error {
		bal, err := retry(func() (uint64, error) {
			model.CountRPC(ctx)
			st, err := pinned.GetAccountState(ctx, electorAddr)
			if err != nil {
				return 0, err
			}
			return uint64(st.Account.Account.Storage.Balance.Grams), nil
		})
		if err == nil {
			electorBalance.SetUint64(bal)
		}
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
			prevBlockBonuses.Set(prev.Bonuses)
			currBlockBonuses.Set(cur.Bonuses)
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

	out := model.Output{
		Block: model.BlockInfo{
			Seqno: blockIDExt.Seqno,
			Time:  blockTime.UTC(),
		},
		ElectionID:            int64(roundSince),
		PrevBlockTotalBonuses: &model.NanoGram{Int: prevBlockBonuses},
		CurrBlockTotalBonuses: &model.NanoGram{Int: currBlockBonuses},
		ElectorBalance:        &model.NanoGram{Int: electorBalance},
		RewardPerBlock:        &model.NanoGram{Int: rewardPerBlock},
	}
	if roundStartBlock > 0 {
		prevID, err := fetchPrevElectionIDForBlock(ctx, client, roundStartBlock)
		if err != nil {
			return nil, fmt.Errorf("fetchPrevElectionIDForBlock: %w", err)
		}
		out.PrevElectionID = prevID
	}
	if roundUntil > 0 && time.Unix(int64(roundUntil), 0).Before(time.Now()) {
		nextID := int64(roundUntil)
		out.NextElectionID = &nextID
	}
	out.ValidationRound = model.RoundInfo{
		Start:      time.Unix(int64(roundSince), 0).UTC(),
		End:        time.Unix(int64(roundUntil), 0).UTC(),
		StartBlock: roundStartBlock,
		EndBlock:   roundEndBlock,
	}

	// Build validator rows.
	rows, totalTrueStake := buildValidatorRows(rd.conf, rd.pools)
	log.Printf("active validators: %d", len(rows))
	out.TotalStake = &model.NanoGram{Int: totalTrueStake}
	log.Printf("total true stake (active validators): %.2f TON", new(big.Float).Quo(new(big.Float).SetInt(totalTrueStake), big.NewFloat(1e9)))

	out.Validators = computeValidatorRewards(ctx, pinned, rows, totalTrueStake, rewardPerBlock, shallow)
	return &out, nil
}
