package litestorage

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) STONfiPools(ctx context.Context, poolIDs []tongo.AccountID) (map[tongo.AccountID]core.STONfiPool, error) {
	pools := make(map[tongo.AccountID]core.STONfiPool)
	for _, poolID := range poolIDs {
		_, value, err := abi.GetPoolData(ctx, s.client, poolID)
		if err != nil {
			return nil, err
		}
		result, ok := value.(abi.GetPoolData_StonfiResult)
		if !ok {
			continue
		}
		token0, err := tongo.AccountIDFromTlb(result.Token0Address)
		if err != nil {
			return nil, err
		}
		if token0 == nil {
			continue
		}
		token1, err := tongo.AccountIDFromTlb(result.Token1Address)
		if err != nil {
			return nil, err
		}
		if token1 == nil {
			continue
		}
		pools[poolID] = core.STONfiPool{
			Token0: *token0,
			Token1: *token1,
		}
	}
	return pools, nil
}
