package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/rewards/model"
)

var errServiceUnavailable = errors.New(http.StatusText(http.StatusServiceUnavailable))

func (h *Handler) GetValidators(ctx context.Context, params oas.GetValidatorsParams) (*oas.ValidatorsResponse, error) {
	timeStart := time.Now()
	if h.rewards == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	var seqno *uint32
	if params.Seqno.Set {
		seqno = &params.Seqno.Value
	}
	var unixtime *uint32
	if params.Unixtime.Set {
		unixtime = &params.Unixtime.Value
	}
	if seqno != nil && unixtime != nil {
		return nil, toError(http.StatusBadRequest, errors.New("seqno and unixtime are mutually exclusive"))
	}
	out, err := h.rewards.FetchPerBlockRewards(ctx, seqno, unixtime, params.Shallow.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := &oas.ValidatorsResponse{
		ResponseTimeMs: out.ResponseTimeMs,
		Block: oas.BlockInfo{
			Seqno: out.Block.Seqno,
			Utime: oas.NewOptInt64(out.Block.Time.Unix()),
		},
		ValidationRound: oas.RoundInfo{
			StartUtime: oas.NewOptInt64(out.ValidationRound.Start.Unix()),
			EndUtime:   oas.NewOptInt64(out.ValidationRound.End.Unix()),
			StartBlock: out.ValidationRound.StartBlock,
			EndBlock:   out.ValidationRound.EndBlock,
		},
		ElectionID: out.ElectionID,
	}
	if out.PrevElectionID != nil {
		res.PrevElectionID = oas.NewOptInt64(*out.PrevElectionID)
	}
	if out.NextElectionID != nil {
		res.NextElectionID = oas.NewOptInt64(*out.NextElectionID)
	}
	if out.ElectorBalance != nil {
		res.ElectorBalance = out.ElectorBalance.Int64()
	}
	if out.TotalStake != nil {
		res.TotalStake = out.TotalStake.Int64()
	}
	if out.RewardPerBlock != nil {
		res.RewardPerBlock = out.RewardPerBlock.Int64()
	}
	if len(out.Validators) != 0 {
		validators := make([]oas.ValidatorRewardEntry, 0, len(out.Validators))
		for _, out := range out.Validators {
			res := oas.ValidatorRewardEntry{
				Rank:                 out.Rank,
				PublicKey:            out.Pubkey,
				Weight:               out.Weight,
				Pool:                 oas.NewOptString(out.Pool),
				PoolType:             oas.NewOptValidatorRewardEntryPoolType(oas.ValidatorRewardEntryPoolType(out.PoolType)),
				OwnerAddress:         oas.NewOptString(out.OwnerAddress),
				ValidatorAddress:     oas.NewOptString(out.ValidatorAddress),
				ValidatorRewardShare: oas.NewOptFloat64(out.ValidatorRewardShare),
				NominatorsCount:      oas.NewOptUint32(out.NominatorsCount),
			}
			if out.EffectiveStake != nil {
				res.EffectiveStake = out.EffectiveStake.Int64()
			}
			if out.Reward != nil {
				res.Reward = out.Reward.Int64()
			}
			if out.ValidatorStake != nil {
				res.ValidatorStake = oas.NewOptInt64(out.ValidatorStake.Int64())
			}
			if out.NominatorsStake != nil {
				res.NominatorsStake = oas.NewOptInt64(out.NominatorsStake.Int64())
			}
			if out.TotalStake != nil {
				res.TotalStake = oas.NewOptInt64(out.TotalStake.Int64())
			}
			if len(out.Nominators) != 0 {
				var nominators []oas.NominatorRewardEntry
				for _, out := range out.Nominators {
					res := oas.NominatorRewardEntry{
						Address: out.Address,
						Weight:  out.Weight,
					}
					if out.Reward != nil {
						res.Reward = out.Reward.Int64()
					}
					if out.EffectiveStake != nil {
						res.EffectiveStake = out.EffectiveStake.Int64()
					}
					if out.Stake != nil {
						res.Stake = out.Stake.Int64()
					}
					nominators = append(nominators, res)
				}
				res.Nominators = nominators
			}
			validators = append(validators, res)
		}
		res.Validators = validators

	}
	res.ResponseTimeMs = time.Since(timeStart).Milliseconds()
	return res, nil
}

func (h *Handler) GetValidationRounds(ctx context.Context, params oas.GetValidationRoundsParams) (*oas.ValidationRoundsResponse, error) {
	timeStart := time.Now()
	if h.rewards == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	query := model.RoundsQuery{}
	count := 0
	if params.ElectionID.Set {
		query.ElectionID = &params.ElectionID.Value
		count++
	}
	if params.Block.Set {
		query.Block = &params.Block.Value
		count++
	}
	if params.Unixtime.Set {
		query.Unixtime = &params.Unixtime.Value
		count++
	}
	if count > 1 {
		return nil, toError(http.StatusBadRequest, errors.New("election_id, block, and unixtime are mutually exclusive"))
	}
	out, err := h.rewards.FetchValidationRounds(ctx, query)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := &oas.ValidationRoundsResponse{
		ResponseTimeMs: out.ResponseTimeMs,
	}
	if len(out.Rounds) != 0 {
		rounds := make([]oas.ValidationRound, 0, len(out.Rounds))
		for _, out := range out.Rounds {
			res := oas.ValidationRound{
				ElectionID: out.ElectionID,
				StartUtime: oas.NewOptInt64(out.Start.Unix()),
				EndUtime:   oas.NewOptInt64(out.End.Unix()),
				StartBlock: out.StartBlock,
				Finished:   out.Finished,
			}
			if out.EndBlock != 0 {
				res.EndBlock = oas.NewOptUint32(out.EndBlock)
			}
			if out.PrevElectionID != nil {
				res.PrevElectionID = oas.NewOptInt64(*out.PrevElectionID)
			}
			if out.NextElectionID != nil {
				res.NextElectionID = oas.NewOptInt64(*out.NextElectionID)
			}
			rounds = append(rounds, res)
		}
		res.Rounds = rounds
	}
	res.ResponseTimeMs = time.Since(timeStart).Milliseconds()
	return res, nil
}

func (h *Handler) GetRoundRewards(ctx context.Context, params oas.GetRoundRewardsParams) (*oas.RoundRewardsResponse, error) {
	timeStart := time.Now()
	if h.rewards == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	query := model.RoundRewardsQuery{}
	count := 0
	if params.ElectionID.Set {
		query.ElectionID = &params.ElectionID.Value
		count++
	}
	if params.Block.Set {
		query.Block = &params.Block.Value
		count++
	}
	if params.Unixtime.Set {
		query.Unixtime = &params.Unixtime.Value
		count++
	}
	if count > 1 {
		return nil, toError(http.StatusBadRequest, errors.New("election_id, block, and unixtime are mutually exclusive"))
	}
	if count == 0 {
		return nil, toError(http.StatusBadRequest, errors.New("one of election_id, block, or unixtime is required"))
	}
	out, err := h.rewards.FetchRoundRewards(ctx, query, params.Shallow.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := &oas.RoundRewardsResponse{
		ResponseTimeMs: out.ResponseTimeMs,
		ElectionID:     out.ElectionID,
		RoundStart:     out.RoundStart,
		RoundEnd:       out.RoundEnd,
		StartBlock:     out.StartBlock,
		EndBlock:       out.EndBlock,
	}
	if out.PrevElectionID != nil {
		res.PrevElectionID = oas.NewOptInt64(*out.PrevElectionID)
	}
	if out.NextElectionID != nil {
		res.NextElectionID = oas.NewOptInt64(*out.NextElectionID)
	}
	if out.TotalBonuses != nil {
		res.TotalBonuses = out.TotalBonuses.Int64()
	}
	if out.TotalStake != nil {
		res.TotalStake = out.TotalStake.Int64()
	}
	if len(out.Validators) != 0 {
		validators := make([]oas.ValidatorRewardEntry, 0, len(out.Validators))
		for _, out := range out.Validators {
			res := oas.ValidatorRewardEntry{
				Rank:                 out.Rank,
				PublicKey:            out.Pubkey,
				Weight:               out.Weight,
				Pool:                 oas.NewOptString(out.Pool),
				PoolType:             oas.NewOptValidatorRewardEntryPoolType(oas.ValidatorRewardEntryPoolType(out.PoolType)),
				OwnerAddress:         oas.NewOptString(out.OwnerAddress),
				ValidatorAddress:     oas.NewOptString(out.ValidatorAddress),
				ValidatorRewardShare: oas.NewOptFloat64(out.ValidatorRewardShare),
				NominatorsCount:      oas.NewOptUint32(out.NominatorsCount),
			}
			if out.EffectiveStake != nil {
				res.EffectiveStake = out.EffectiveStake.Int64()
			}
			if out.Reward != nil {
				res.Reward = out.Reward.Int64()
			}
			if out.ValidatorStake != nil {
				res.ValidatorStake = oas.NewOptInt64(out.ValidatorStake.Int64())
			}
			if out.NominatorsStake != nil {
				res.NominatorsStake = oas.NewOptInt64(out.NominatorsStake.Int64())
			}
			if out.TotalStake != nil {
				res.TotalStake = oas.NewOptInt64(out.TotalStake.Int64())
			}
			if len(out.Nominators) != 0 {
				nominators := make([]oas.NominatorRewardEntry, 0, len(out.Nominators))
				for _, out := range out.Nominators {
					res := oas.NominatorRewardEntry{
						Address: out.Address,
						Weight:  out.Weight,
					}
					if out.Reward != nil {
						res.Reward = out.Reward.Int64()
					}
					if out.EffectiveStake != nil {
						res.EffectiveStake = out.EffectiveStake.Int64()
					}
					if out.Stake != nil {
						res.Stake = out.Stake.Int64()
					}
					nominators = append(nominators, res)
				}
				res.Nominators = nominators
			}
			validators = append(validators, res)
		}
		res.Validators = validators
	}
	res.ResponseTimeMs = time.Since(timeStart).Milliseconds()
	return res, nil
}
