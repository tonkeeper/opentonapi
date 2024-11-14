package litestorage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arnac-io/opentonapi/pkg/core"
	"github.com/tonkeeper/tonapi-go"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

func (s *LiteStorage) STONfiPools(ctx context.Context, poolIDs []core.STONfiPoolID) (map[tongo.AccountID]core.STONfiPool, error) {
	pools := make(map[tongo.AccountID]core.STONfiPool)
	for _, poolID := range poolIDs {
		params := tonapi.ExecGetMethodForBlockchainAccountParams{
			AccountID:  poolID.ID.String(),
			MethodName: "get_pool_data",
		}
		res, err := s.tonapiClient.ExecGetMethodForBlockchainAccount(ctx, params)
		if err != nil {
			continue
		}

		rawResult := map[string]interface{}{}
		err = json.Unmarshal(res.Decoded, &rawResult)
		if err != nil {
			continue
		}

		// Fields in case of STONfi v1
		token0Address := rawResult["token0_address"]
		token1Address := rawResult["token1_address"]
		if token0Address == nil || token1Address == nil {
			// Fields in case of STONfi v2
			token0Address = rawResult["token0_wallet_address"]
			token1Address = rawResult["token1_wallet_address"]
		}

		if token0Address == nil || token1Address == nil {
			s.logger.Info(fmt.Sprintf("Looks like %s isn't really a STONfi pool", poolID.ID.String()))
			continue
		}
		token0AddressString, ok := token0Address.(string)
		if !ok {
			continue
		}
		token1AddressString, ok := token1Address.(string)
		if !ok {
			continue
		}

		token0, err := ton.ParseAccountID(token0AddressString)
		if err != nil {
			continue
		}
		token1, err := ton.ParseAccountID(token1AddressString)
		if err != nil {
			continue
		}
		pools[poolID.ID] = core.STONfiPool{
			Token0: token0,
			Token1: token1,
		}
	}
	return pools, nil
}
