package cache

import (
	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lru"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var cacheMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_cache",
		Help: "",
	},
	[]string{
		"name",
		"result",
	},
)

type Cache[K comparable, V any] struct {
	cache      *cache.Cache[K, V]
	metricName string
	size       int
}

func NewLRUCache[K comparable, V any](size int, metricName string) Cache[K, V] {
	return Cache[K, V]{
		cache:      cache.New(cache.AsLRU[K, V](lru.WithCapacity(size))),
		metricName: metricName,
		size:       size,
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	val, ok := c.cache.Get(key)
	if ok {
		cacheMetrics.WithLabelValues(c.metricName, "hit").Inc()
		return val, ok
	}
	cacheMetrics.WithLabelValues(c.metricName, "miss").Inc()
	return val, ok
}

func (c *Cache[K, V]) Set(key K, val V, opts ...cache.ItemOption) {
	c.cache.Set(key, val, opts...)
}

// Keys returns the keys of the cache. the order is relied on algorithms.
func (c *Cache[K, V]) Keys() []K {
	return c.cache.Keys()
}

var WithExpiration = cache.WithExpiration
