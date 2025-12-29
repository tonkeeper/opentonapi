package cache

import (
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lru"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cacheRequestsStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_cache",
			Help: "",
		},
		[]string{
			"name",
			"result",
		},
	)
	cacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "db_cache_fullness"}, []string{"name"})
)

type Cache[K comparable, V any] struct {
	cache      *cache.Cache[K, V]
	metricName string
}

func NewLRUCache[K comparable, V any](size int, metricName string) Cache[K, V] {

	c := Cache[K, V]{
		cache:      cache.New(cache.AsLRU[K, V](lru.WithCapacity(size))),
		metricName: metricName,
	}
	go func() {
		for {
			time.Sleep(5 * time.Second)
			cacheSize.WithLabelValues(metricName).Add(float64(c.cache.Len()) / float64(size))
		}
	}()
	return c
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	val, ok := c.cache.Get(key)
	if ok {
		cacheRequestsStatus.WithLabelValues(c.metricName, "hit").Inc()
		return val, ok
	}
	cacheRequestsStatus.WithLabelValues(c.metricName, "miss").Inc()
	return val, ok
}

func (c *Cache[K, V]) Set(key K, val V, opts ...cache.ItemOption) {
	c.cache.Set(key, val, opts...)
}

func (c *Cache[K, V]) Delete(key K) {
	c.cache.Delete(key)
}

// Keys returns the keys of the cache. the order is relied on algorithms.
func (c *Cache[K, V]) Keys() []K {
	return c.cache.Keys()
}

var WithExpiration = cache.WithExpiration
