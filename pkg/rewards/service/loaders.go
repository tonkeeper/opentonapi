package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	dataloader "github.com/graph-gophers/dataloader/v7"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/opentonapi/pkg/rewards/model"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Cache implements the dataloader.Cache interface
type cache[K comparable, V any] struct {
	*lru.Cache[K, V]
}

// Get gets an item from the cache
func (c *cache[K, V]) Get(_ context.Context, key K) (V, bool) {
	v, ok := c.Cache.Get(key)
	if ok {
		return v, ok
	}
	return v, ok
}

// Set sets an item in the cache
func (c *cache[K, V]) Set(_ context.Context, key K, value V) {
	c.Cache.Add(key, value)
}

// Delete deletes an item in the cache
func (c *cache[K, V]) Delete(_ context.Context, key K) bool {
	if c.Cache.Contains(key) {
		c.Cache.Remove(key)
		return true
	}
	return false
}

// Clear clears the cache
func (c *cache[K, V]) Clear() {
	c.Cache.Purge()
}

type loadersKey struct{}

// blockResult holds the cached result of a lookupMasterchainBlock call.
type blockResult struct {
	ext  ton.BlockIDExt
	time time.Time
}

// configParamsKey is the cache key for GetConfigParams calls.
type configParamsKey struct {
	mode   liteapi.ConfigMode
	params string // fmt.Sprint of []uint32 — slices aren't comparable
	seqno  uint32
}

func newConfigParamsKey(mode liteapi.ConfigMode, paramList []uint32, seqno uint32) configParamsKey {
	b, _ := json.Marshal(paramList)
	return configParamsKey{mode: mode, params: string(b), seqno: seqno}
}

// parseParamList reconstructs []uint32 from the JSON representation (e.g. "[34]" → []uint32{34}).
func parseParamList(s string) ([]uint32, error) {
	var result []uint32
	err := json.Unmarshal([]byte(s), &result)
	return result, err
}

// loaders holds per-request dataloaders.
type loaders struct {
	blockByUtime *dataloader.Loader[uint32, ton.BlockIDExt]
	blockBySeqno *dataloader.Loader[uint32, blockResult]
	configParams *dataloader.Loader[configParamsKey, string]
}

var loadersMu sync.Mutex

var globalLoaders *loaders

// WithLoaders creates a new context with dataloaders backed by the given client.
func WithLoaders(ctx context.Context, client LiteClient) context.Context {
	loadersMu.Lock()
	defer loadersMu.Unlock()
	if globalLoaders != nil {
		return context.WithValue(ctx, loadersKey{}, globalLoaders)
	}

	blockSeqnoLru, err := lru.New[uint32, dataloader.Thunk[ton.BlockIDExt]](1000)
	if err != nil {
		panic(err)
	}
	blockSeqnoCache := &cache[uint32, dataloader.Thunk[ton.BlockIDExt]]{Cache: blockSeqnoLru}

	blockUtimeLru, err := lru.New[uint32, dataloader.Thunk[blockResult]](1000)
	if err != nil {
		panic(err)
	}
	blockUtimeCache := &cache[uint32, dataloader.Thunk[blockResult]]{Cache: blockUtimeLru}

	configParamsLru, err := lru.New[configParamsKey, dataloader.Thunk[string]](1000)
	if err != nil {
		panic(err)
	}
	configParamsCache := &cache[configParamsKey, dataloader.Thunk[string]]{Cache: configParamsLru}

	utimeBatch := func(ctx context.Context, keys []uint32) []*dataloader.Result[ton.BlockIDExt] {
		results := make([]*dataloader.Result[ton.BlockIDExt], len(keys))
		var wg sync.WaitGroup
		wg.Add(len(keys))
		for i, utime := range keys {
			go func() {
				defer wg.Done()
				blockID := ton.BlockID{
					Workchain: -1,
					Shard:     0x8000000000000000,
				}
				innerCtx, _ := WithRetryExclude(ctx)
				ext, err := retryWithExclude(innerCtx, func() (ton.BlockIDExt, error) {
					model.CountRPC(ctx)
					ext, _, err := client.LookupBlock(ctx, blockID, 4, nil, &utime)
					if err != nil {
						return ton.BlockIDExt{}, err
					}
					return ext, nil
				})
				if err != nil {
					results[i] = &dataloader.Result[ton.BlockIDExt]{Error: dataloader.NewSkipCacheError(fmt.Errorf("lookupMasterchainBlockByUtime(%d): %w", utime, err))}
				} else {
					results[i] = &dataloader.Result[ton.BlockIDExt]{Data: ext}
				}
			}()
		}
		wg.Wait()
		return results
	}

	seqnoBatch := func(ctx context.Context, keys []uint32) []*dataloader.Result[blockResult] {
		results := make([]*dataloader.Result[blockResult], len(keys))
		var wg sync.WaitGroup
		wg.Add(len(keys))
		for i, seqno := range keys {
			go func() {
				defer wg.Done()
				blockID := ton.BlockID{
					Workchain: -1,
					Shard:     0x8000000000000000,
					Seqno:     seqno,
				}
				innerCtx, _ := WithRetryExclude(ctx)
				r, err := retryWithExclude(innerCtx, func() (blockResult, error) {
					model.CountRPC(ctx)
					ext, info, err := client.LookupBlock(ctx, blockID, 1, nil, nil)
					if err != nil {
						return blockResult{}, err
					}
					return blockResult{ext, time.Unix(int64(info.GenUtime), 0)}, nil
				})
				if err != nil {
					results[i] = &dataloader.Result[blockResult]{Error: dataloader.NewSkipCacheError(fmt.Errorf("lookupMasterchainBlock(%d): %w", seqno, err))}
				} else {
					results[i] = &dataloader.Result[blockResult]{Data: r}
				}
			}()
		}
		wg.Wait()
		return results
	}

	l := &loaders{
		blockByUtime: dataloader.NewBatchedLoader(utimeBatch,
			dataloader.WithInputCapacity[uint32, ton.BlockIDExt](1),
			dataloader.WithCache(blockSeqnoCache),
		),
		blockBySeqno: dataloader.NewBatchedLoader(seqnoBatch,
			dataloader.WithInputCapacity[uint32, blockResult](1),
			dataloader.WithCache(blockUtimeCache),
		),
	}

	configBatch := func(ctx context.Context, keys []configParamsKey) []*dataloader.Result[string] {
		results := make([]*dataloader.Result[string], len(keys))
		var wg sync.WaitGroup
		wg.Add(len(keys))
		for i, key := range keys {
			go func() {
				defer wg.Done()
				paramList, err := parseParamList(key.params)
				if err != nil {
					results[i] = &dataloader.Result[string]{Error: dataloader.NewSkipCacheError(fmt.Errorf("parseParamList(%q): %w", key.params, err))}
					return
				}

				block, _, err := lookupMasterchainBlock(ctx, client, key.seqno)
				if err != nil {
					results[i] = &dataloader.Result[string]{Error: dataloader.NewSkipCacheError(fmt.Errorf("lookupMasterchainBlock(%q): %w", key.params, err))}
					return
				}

				pinned := client.WithBlock(block)

				params, err := retry(func() (tlb.ConfigParams, error) {
					model.CountRPC(ctx)
					return pinned.GetConfigParams(ctx, key.mode, paramList)
				})

				if err != nil {
					results[i] = &dataloader.Result[string]{Error: dataloader.NewSkipCacheError(fmt.Errorf("GetConfigParams(%d, %s): %w", key.seqno, key.params, err))}
				} else {
					cell := boc.NewCell()

					if err := tlb.Marshal(cell, &params); err != nil {
						results[i] = &dataloader.Result[string]{Error: dataloader.NewSkipCacheError(fmt.Errorf("GetConfigParams(%d, %s) marshal: %w", key.seqno, key.params, err))}
						return
					}

					str, err := cell.ToBoc()
					if err != nil {
						results[i] = &dataloader.Result[string]{Error: dataloader.NewSkipCacheError(fmt.Errorf("GetConfigParams(%d, %s) to boc: %w", key.seqno, key.params, err))}
						return
					}

					results[i] = &dataloader.Result[string]{Data: string(str)}
				}
			}()
		}
		wg.Wait()
		return results
	}
	l.configParams = dataloader.NewBatchedLoader(
		configBatch,
		dataloader.WithInputCapacity[configParamsKey, string](1),
		dataloader.WithCache(configParamsCache),
	)
	globalLoaders = l
	return context.WithValue(ctx, loadersKey{}, l)
}

func getLoaders(ctx context.Context) *loaders {
	l, _ := ctx.Value(loadersKey{}).(*loaders)
	return l
}

// cachedGetConfigParams fetches config params using the dataloader when available.
// pinnedClient is used as fallback when no loader is in context; seqno is the cache key.
func cachedGetConfigParams(ctx context.Context, pinnedClient LiteClient, mode liteapi.ConfigMode, paramList []uint32, seqno uint32) (tlb.ConfigParams, error) {
	if l := getLoaders(ctx); l != nil {
		key := newConfigParamsKey(mode, paramList, seqno)
		thunk := l.configParams.Load(ctx, key)
		result, err := thunk()
		if err != nil {
			return tlb.ConfigParams{}, err
		}
		cell, err := boc.DeserializeBoc([]byte(result))
		if err != nil {
			return tlb.ConfigParams{}, err
		}

		var copyConfigParams tlb.ConfigParams
		if err := tlb.Unmarshal(cell[0], &copyConfigParams); err != nil {
			return tlb.ConfigParams{}, err
		}
		return copyConfigParams, err
	}

	// Fallback: no loader in context.
	return retry(func() (tlb.ConfigParams, error) {
		model.CountRPC(ctx)
		return pinnedClient.GetConfigParams(ctx, mode, paramList)
	})
}

// lookupMasterchainBlockByUtime resolves a unix timestamp to the nearest masterchain block.
// Uses the per-request dataloader when available, falling back to a direct RPC call.
func lookupMasterchainBlockByUtime(ctx context.Context, client LiteClient, utime uint32) (ton.BlockIDExt, error) {
	if l := getLoaders(ctx); l != nil {
		thunk := l.blockByUtime.Load(ctx, utime)
		return thunk()
	}

	// Fallback: no loader in context.
	blockID := ton.BlockID{
		Workchain: -1,
		Shard:     0x8000000000000000,
	}
	ctx, _ = WithRetryExclude(ctx)
	return retryWithExclude(ctx, func() (ton.BlockIDExt, error) {
		model.CountRPC(ctx)
		ext, _, err := client.LookupBlock(ctx, blockID, 4, nil, &utime)
		if err != nil {
			return ton.BlockIDExt{}, err
		}
		return ext, nil
	})
}

// lookupMasterchainBlock resolves a seqno to a BlockIDExt and returns the block time.
// Uses the per-request dataloader when available, falling back to a direct RPC call.
func lookupMasterchainBlock(ctx context.Context, client LiteClient, seqno uint32) (ton.BlockIDExt, time.Time, error) {
	if l := getLoaders(ctx); l != nil {
		thunk := l.blockBySeqno.Load(ctx, seqno)
		r, err := thunk()
		return r.ext, r.time, err
	}

	// Fallback: no loader in context.
	blockID := ton.BlockID{
		Workchain: -1,
		Shard:     0x8000000000000000,
		Seqno:     seqno,
	}
	ctx, _ = WithRetryExclude(ctx)
	r, err := retryWithExclude(ctx, func() (blockResult, error) {
		model.CountRPC(ctx)
		ext, info, err := client.LookupBlock(ctx, blockID, 1, nil, nil)
		if err != nil {
			return blockResult{}, err
		}
		return blockResult{ext, time.Unix(int64(info.GenUtime), 0)}, nil
	})
	return r.ext, r.time, err
}
