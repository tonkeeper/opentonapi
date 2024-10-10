package litestorage

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
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
			pools[poolID.ID] = core.STONfiPool{
				Token0: *token0,
				Token1: *token1,
			}
		case abi.GetPoolData_StonfiV2Result:
			token0, err := tongo.AccountIDFromTlb(result.Token0WalletAddress)
			if err != nil {
				return nil, err
			}
			if token0 == nil {
				continue
			}
			token1, err := tongo.AccountIDFromTlb(result.Token1WalletAddress)
			if err != nil {
				return nil, err
			}
			if token1 == nil {
				continue
			}
			pools[poolID.ID] = core.STONfiPool{
				Token0: *token0,
				Token1: *token1,
			}
		}
	}
	return pools, nil
}
