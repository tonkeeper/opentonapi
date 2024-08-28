package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/arnac-io/opentonapi/pkg/cache"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tep64"
)

func (mc *metadataCache) getCollectionMeta(ctx context.Context, a tongo.AccountID) (tep64.Metadata, bool) {
	m, ok := mc.collectionsCache.Get(a)
	if ok {
		return m, ok
	}
	collection, err := mc.storage.GetNftCollectionByCollectionAddress(ctx, a)
	if err != nil {
		return tep64.Metadata{}, false
	}
	m = metaMapToStruct(collection.Metadata)
	mc.collectionsCache.Set(a, m, cache.WithExpiration(time.Minute*10))
	return m, ok
}

func metaMapToStruct(m map[string]interface{}) tep64.Metadata { //todo: rewrite to k, v := range m {switch k
	var m2 tep64.Metadata
	b, _ := json.Marshal(m)
	json.Unmarshal(b, &m2)
	return m2
}

func (mc *metadataCache) getJettonMeta(ctx context.Context, a tongo.AccountID) (tep64.Metadata, bool) {
	m, ok := mc.jettonsCache.Get(a)
	if ok {
		return m, true
	}
	m, err := mc.storage.GetJettonMasterMetadata(ctx, a)
	if err != nil {
		return m, false
	}
	mc.jettonsCache.Set(a, m, cache.WithExpiration(time.Minute*10))
	return m, true
}
