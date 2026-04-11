package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

var errServiceUnavailable = errors.New(http.StatusText(http.StatusServiceUnavailable))
var errBothElectionIDAndBlockSet = errors.New("election_id and block are mutually exclusive")
var errNoneElectionIDAndBlockSet = errors.New("one of election_id or block is required")

func (h *Handler) GetValidators(ctx context.Context, params oas.GetValidatorsParams) (*oas.ValidatorsResponse, error) {
	if h.validation == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	res, err := h.validation.FetchPerBlockRewards(ctx, params.Seqno.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return res, nil
}

func (h *Handler) GetValidationRounds(ctx context.Context, params oas.GetValidationRoundsParams) (*oas.ValidationRoundsResponse, error) {
	if h.validation == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	start := time.Now()
	rounds, err := h.validation.FetchValidationRounds(ctx, params)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := oas.ValidationRoundsResponse{
		ResponseTimeMs: time.Since(start).Milliseconds(),
		Rounds:         rounds,
	}
	return &res, nil
}

func (h *Handler) GetRoundRewards(ctx context.Context, params oas.GetRoundRewardsParams) (*oas.RoundRewardsResponse, error) {
	if h.validation == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	if params.ElectionID.Set && params.Block.Set {
		return nil, toError(http.StatusBadRequest, errBothElectionIDAndBlockSet)
	}
	if !params.ElectionID.Set && !params.Block.Set {
		return nil, toError(http.StatusBadRequest, errNoneElectionIDAndBlockSet)
	}
	res, err := h.validation.FetchRoundRewards(ctx, params)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return res, nil
}
