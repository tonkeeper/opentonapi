package litestorage

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

func (s *LiteStorage) STONfiPools(ctx context.Context, poolIDs []core.STONfiPoolID) (map[tongo.AccountID]core.STONfiPool, error) {
	pools := make(map[tongo.AccountID]core.STONfiPool)
	for _, poolID := range poolIDs {
		_, value, err := abi.GetPoolData(ctx, s.executor, poolID.ID)
		if err != nil {
			return nil, err
		}
		switch result := value.(type) {
		case abi.GetPoolData_StonfiResult:
			pool, err := tokensToPool(result.Token0Address, result.Token1Address)
			if err != nil {
				return nil, err
			}
			if pool == nil {
				continue
			}
			pools[poolID.ID] = *pool
		case abi.GetPoolData_StonfiV2Result:
			pool, err := tokensToPool(result.Token0WalletAddress, result.Token1WalletAddress)
			if err != nil {
				return nil, err
			}
			if pool == nil {
				continue
			}
			pools[poolID.ID] = *pool
		case abi.GetPoolData_StonfiV2StableswapResult:
			pool, err := tokensToPool(result.Token0WalletAddress, result.Token1WalletAddress)
			if err != nil {
				return nil, err
			}
			if pool == nil {
				continue
			}
			pools[poolID.ID] = *pool
		case abi.GetPoolData_StonfiV2WeightedStableswapResult:
			pool, err := tokensToPool(result.Token0WalletAddress, result.Token1WalletAddress)
			if err != nil {
				return nil, err
			}
			if pool == nil {
				continue
			}
			pools[poolID.ID] = *pool
		}
	}
	return pools, nil
}

func tokensToPool(token0Addr, token1Addr tlb.MsgAddress) (*core.STONfiPool, error) {
	token0, err := tongo.AccountIDFromTlb(token0Addr)
	if err != nil {
		return nil, err
	}
	if token0 == nil {
		return nil, nil
	}
	token1, err := tongo.AccountIDFromTlb(token1Addr)
	if err != nil {
		return nil, err
	}
	if token1 == nil {
		return nil, nil
	}
	return &core.STONfiPool{
		Token0: *token0,
		Token1: *token1,
	}, nil
}
