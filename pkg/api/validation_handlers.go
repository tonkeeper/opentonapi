package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

var errServiceUnavailable = errors.New(http.StatusText(http.StatusServiceUnavailable))

func (h *Handler) GetValidators(ctx context.Context, params oas.GetValidatorsParams) (*oas.ValidatorsResponse, error) {
	if h.validation == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	return h.validation.FetchPerBlockRewards(ctx, params.Seqno.Value)
}

func (h *Handler) GetValidationRounds(ctx context.Context, params oas.GetValidationRoundsParams) (*oas.ValidationRoundsResponse, error) {
	if h.validation == nil {
		return nil, toError(http.StatusServiceUnavailable, errServiceUnavailable)
	}
	start := time.Now()
	rounds, err := h.validation.FetchValidationRounds(ctx, params)
	if err != nil {
		return nil, err
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
	return h.validation.FetchRoundRewards(ctx, params)
}
