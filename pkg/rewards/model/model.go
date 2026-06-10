package model

import (
	"time"
)

// JSON output types.

type Output struct {
	ResponseTimeMs        int64             `json:"response_time_ms"`
	Block                 BlockInfo         `json:"block"`
	ValidationRound       RoundInfo         `json:"validation_round"`
	ElectionID            int64             `json:"election_id"`
	PrevElectionID        *int64            `json:"prev_election_id,omitempty"`
	NextElectionID        *int64            `json:"next_election_id,omitempty"`
	ElectorBalance        *NanoGram         `json:"elector_balance"`
	TotalStake            *NanoGram         `json:"total_stake"`
	RewardPerBlock        *NanoGram         `json:"reward_per_block"`
	PrevBlockTotalBonuses *NanoGram         `json:"prev_block_total_bonuses"`
	CurrBlockTotalBonuses *NanoGram         `json:"curr_block_total_bonuses"`
	Validators            []ValidatorReward `json:"validators"`
}

type BlockInfo struct {
	Seqno uint32    `json:"seqno"`
	Time  time.Time `json:"time"`
}

type RoundInfo struct {
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	StartBlock uint32    `json:"start_block"`
	EndBlock   uint32    `json:"end_block,omitempty"`
}

type ValidationRound struct {
	ElectionID     int64     `json:"election_id"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	StartBlock     uint32    `json:"start_block"`
	EndBlock       uint32    `json:"end_block,omitempty"`
	Finished       bool      `json:"finished"`
	PrevElectionID *int64    `json:"prev_election_id,omitempty"`
	NextElectionID *int64    `json:"next_election_id,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type ValidationRoundsOutput struct {
	ResponseTimeMs int64             `json:"response_time_ms"`
	Rounds         []ValidationRound `json:"rounds"`
	Error          string            `json:"error,omitempty"`
}

// RoundsQuery holds query parameters for the validation-rounds endpoint.
type RoundsQuery struct {
	ElectionID *int64
	Block      *uint32
	Unixtime   *uint32
}

// RoundRewardsQuery holds query parameters for the round-rewards endpoint.
type RoundRewardsQuery struct {
	Block      *uint32
	ElectionID *int64
	Unixtime   *uint32
}

// RoundRewardsOutput is the response for the round-rewards endpoint.
type RoundRewardsOutput struct {
	ResponseTimeMs int64             `json:"response_time_ms"`
	ElectionID     int64             `json:"election_id"`
	PrevElectionID *int64            `json:"prev_election_id,omitempty"`
	NextElectionID *int64            `json:"next_election_id,omitempty"`
	RoundStart     time.Time         `json:"round_start"`
	RoundEnd       time.Time         `json:"round_end"`
	StartBlock     uint32            `json:"start_block"`
	EndBlock       uint32            `json:"end_block"`
	TotalBonuses   *NanoGram         `json:"total_bonuses"`
	TotalStake     *NanoGram         `json:"total_stake"`
	Validators     []ValidatorReward `json:"validators"`
	Error          string            `json:"error,omitempty"`
}

// ValidatorReward holds per-validator reward data for a finished round.
type ValidatorReward struct {
	Rank                 int               `json:"rank"`
	Pubkey               string            `json:"pubkey"`
	EffectiveStake       *NanoGram         `json:"effective_stake"`
	Weight               float64           `json:"weight"`
	Reward               *NanoGram         `json:"reward"`
	Pool                 string            `json:"pool,omitempty"`
	OwnerAddress         string            `json:"owner_address,omitempty"`
	ValidatorAddress     string            `json:"validator_address,omitempty"`
	PoolType             string            `json:"pool_type,omitempty"`
	ValidatorStake       *NanoGram         `json:"validator_stake,omitempty"`
	NominatorsStake      *NanoGram         `json:"nominators_stake,omitempty"`
	TotalStake           *NanoGram         `json:"total_stake,omitempty"`
	ValidatorRewardShare float64           `json:"validator_reward_share,omitempty"`
	NominatorsCount      uint32            `json:"nominators_count,omitempty"`
	Nominators           []NominatorReward `json:"nominators,omitempty"`
}

// NominatorReward holds per-nominator reward data for a finished round.
type NominatorReward struct {
	Address        string    `json:"address"`
	Weight         float64   `json:"weight"`
	Reward         *NanoGram `json:"reward"`
	EffectiveStake *NanoGram `json:"effective_stake"`
	Stake          *NanoGram `json:"stake"`
}
