package defi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStonfiPoolsResponseFormat(t *testing.T) {
	sample := `{
		"pool_list": [
			{
				"lp_balance": "1234567890",
				"pool_address": "EQBZo_SgxgWMETkGinFvMxiShI3bvhkLHTXhMsEiD-tUBAfP"
			},
			{
				"lp_balance": "0",
				"pool_address": "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs"
			}
		]
	}`
	var resp stonfiWalletPoolsResponse
	require.NoError(t, json.Unmarshal([]byte(sample), &resp))
	require.Len(t, resp.PoolList, 2)
	require.Equal(t, "1234567890", resp.PoolList[0].LpBalance)
	require.Equal(t, "EQBZo_SgxgWMETkGinFvMxiShI3bvhkLHTXhMsEiD-tUBAfP", resp.PoolList[0].Address)
}

func TestStonfiPoolsResponseEmpty(t *testing.T) {
	var resp stonfiWalletPoolsResponse
	require.NoError(t, json.Unmarshal([]byte(`{"pool_list":[]}`), &resp))
	require.Empty(t, resp.PoolList)
}

func TestStonfiPoolsResponseNull(t *testing.T) {
	var resp stonfiWalletPoolsResponse
	require.NoError(t, json.Unmarshal([]byte(`{"pool_list":null}`), &resp))
	require.Empty(t, resp.PoolList)
}

func TestStonfiFarmsResponseFormat(t *testing.T) {
	sample := `{
		"farm_list": [
			{
				"pool_address": "EQBZo_SgxgWMETkGinFvMxiShI3bvhkLHTXhMsEiD-tUBAfP",
				"nft_infos": [
					{
						"staked_tokens": "9876543210",
						"status": "active"
					}
				]
			}
		]
	}`
	var resp stonfiWalletFarmsResponse
	require.NoError(t, json.Unmarshal([]byte(sample), &resp))
	require.Len(t, resp.FarmList, 1)
	require.Len(t, resp.FarmList[0].NftInfos, 1)
	require.Equal(t, "9876543210", resp.FarmList[0].NftInfos[0].StakedTokens)
	require.Equal(t, "EQBZo_SgxgWMETkGinFvMxiShI3bvhkLHTXhMsEiD-tUBAfP", resp.FarmList[0].PoolAddress)
}

func TestStonfiFarmsResponseEmpty(t *testing.T) {
	var resp stonfiWalletFarmsResponse
	require.NoError(t, json.Unmarshal([]byte(`{"farm_list":null}`), &resp))
	require.Empty(t, resp.FarmList)
}

func TestSwapCoffeeResponseFormat(t *testing.T) {
	sample := `{
		"pools": [
			{
				"pool_address": "EQBZo_SgxgWMETkGinFvMxiShI3bvhkLHTXhMsEiD-tUBAfP",
				"liquidity": {
					"user_amount": "500000000"
				}
			},
			{
				"pool_address": "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs",
				"liquidity": {
					"user_amount": "0"
				}
			}
		]
	}`
	var resp swapCoffeeResponse
	require.NoError(t, json.Unmarshal([]byte(sample), &resp))
	require.Len(t, resp.Pools, 2)
	require.Equal(t, "500000000", resp.Pools[0].Liquidity.UserAmount)
	require.Equal(t, "EQBZo_SgxgWMETkGinFvMxiShI3bvhkLHTXhMsEiD-tUBAfP", resp.Pools[0].PoolAddress)
}

func TestSwapCoffeeResponseEmpty(t *testing.T) {
	var resp swapCoffeeResponse
	require.NoError(t, json.Unmarshal([]byte(`{"pools":null}`), &resp))
	require.Empty(t, resp.Pools)
}

func TestSwapCoffeeMissingLiquidity(t *testing.T) {
	sample := `{"pools": [{"pool_address": "EQBZo_SgxgWMETkGinFvMxiShI3bvhkLHTXhMsEiD-tUBAfP"}]}`
	var resp swapCoffeeResponse
	require.NoError(t, json.Unmarshal([]byte(sample), &resp))
	require.Len(t, resp.Pools, 1)
	require.Equal(t, "", resp.Pools[0].Liquidity.UserAmount)
}
