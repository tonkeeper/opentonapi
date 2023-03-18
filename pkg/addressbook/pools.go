package addressbook

import (
	_ "embed"
	"encoding/json"
	"go.uber.org/zap"
)

//go:embed tf_pools.json
var poolJson []byte

func getPools(log *zap.Logger) []TFPoolInfo {
	var pools []TFPoolInfo
	err := json.Unmarshal(poolJson, &pools)
	if err != nil {
		log.Fatal("unmarshal pools", zap.Error(err))
	}
	return pools
}
