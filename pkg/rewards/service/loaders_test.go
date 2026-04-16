package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"

	lru "github.com/hashicorp/golang-lru/v2"
)

func TestConfigParamsLRUCacheSetGet(t *testing.T) {
	t.Parallel()

	lruk, err := lru.New[configParamsKey, tlb.ConfigParams](16)
	if err != nil {
		t.Fatalf("lru.New: %v", err)
	}
	c := &cache[configParamsKey, tlb.ConfigParams]{Cache: lruk}

	// should return same value for same key
	key := newConfigParamsKey(liteapi.ConfigMode(0), []uint32{34}, 42)
	key2 := newConfigParamsKey(liteapi.ConfigMode(0), []uint32{34}, 42)

	// should return nothing
	key3 := newConfigParamsKey(liteapi.ConfigMode(0), []uint32{34}, 44)
	want := tlb.ConfigParams{}

	ctx := context.Background()
	c.Set(ctx, key, want)

	gotThunk, ok := c.Get(ctx, key)
	if !ok {
		t.Fatal("Get: expected cache hit")
	}

	if !reflect.DeepEqual(gotThunk, want) {
		t.Fatalf("got %+v, want %+v", gotThunk, want)
	}

	gotThunk, ok = c.Get(ctx, key2)
	if !ok {
		t.Fatal("Get: expected cache hit")
	}

	gotThunk, ok = c.Get(ctx, key3)
	if ok {
		t.Fatal("Get: expected cache miss")
	}
}
