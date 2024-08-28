package litestorage

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto"
	cache "github.com/eko/gocache/v3/cache"
	store "github.com/eko/gocache/v3/store"
)

var ErrorNotFound = errors.New("key not found")

type ICache[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T, expiration time.Duration) error
	SetMany(ctx context.Context, items map[string]T, expiration time.Duration) error
}

type InMemoryConfig struct {
	MaxItems int64
}

type InMemoryCache[T any] struct {
	cache *cache.Cache[T]
}

func (c *InMemoryCache[T]) Set(ctx context.Context, key string, value T, expiration time.Duration) error {
	return c.cache.Set(ctx, key, value, store.WithCost(1), store.WithExpiration(expiration))
}

func (c *InMemoryCache[T]) SetMany(ctx context.Context, items map[string]T, expiration time.Duration) error {
	for key, value := range items {
		err := c.Set(ctx, key, value, expiration)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *InMemoryCache[T]) Get(ctx context.Context, key string) (T, error) {
	value, err := c.cache.Get(ctx, key)
	if err != nil {
		var resultObject T
		if strings.Contains(err.Error(), "value not found") {
			return resultObject, ErrorNotFound
		}
		return resultObject, err
	}
	return value, nil
}

func (c *InMemoryCache[T]) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}

func NewCache[T any](maxItems int64) (ICache[T], error) {
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * maxItems,
		MaxCost:     maxItems,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}

	ristrettoStore := store.NewRistretto(ristrettoCache)
	cacheManager := InMemoryCache[T]{cache.New[T](ristrettoStore)}

	return &cacheManager, nil

}
