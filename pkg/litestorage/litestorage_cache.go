package litestorage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"go.etcd.io/bbolt"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

type Cache[K, V any] interface {
	Store(key K, value V)
	Load(key K) (value V, ok bool)
}

var boltBucket = []byte("cache")

// BoltCache is a bbolt-backed key/value cache with [string, []byte] types.
// bbolt is a pure-Go embedded database; no CGO or shared libraries are required.
type BoltCache struct {
	db *bbolt.DB
}

// NewBoltCache opens (or creates) a bbolt database at the given file path.
func NewBoltCache(path string) (*BoltCache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(boltBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &BoltCache{db: db}, nil
}

func (c *BoltCache) Store(key string, value []byte) {
	if err := c.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(boltBucket).Put([]byte(key), value)
	}); err != nil {
		panic(fmt.Sprintf("BoltCache: Put %s: %v", key, err))
	}
}

func (c *BoltCache) Load(key string) ([]byte, bool) {
	var result []byte
	if err := c.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(boltBucket).Get([]byte(key))
		if v != nil {
			result = make([]byte, len(v))
			copy(result, v)
		}
		return nil
	}); err != nil {
		return nil, false
	}
	return result, result != nil
}

// BytesDiskCache is a Cache[K, []byte] backed by a BoltCache.
type BytesDiskCache[K fmt.Stringer] struct {
	cache *BoltCache
}

func (dc *BytesDiskCache[K]) Store(key K, value []byte) {
	dc.cache.Store(key.String(), value)
}

func (dc *BytesDiskCache[K]) Load(key K) ([]byte, bool) {
	return dc.cache.Load(key.String())
}

// JsonTlbDiskCache is a Cache[K, V] that serialises values as JSON-encoded TLB cells,
// backed by a BoltCache.
type JsonTlbDiskCache[K fmt.Stringer, V any] struct {
	cache *BoltCache
}

func (dc *JsonTlbDiskCache[K, V]) Store(key K, value V) {
	cell := boc.NewCell()
	if err := tlb.Marshal(cell, value); err != nil {
		panic(err)
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(cell); err != nil {
		panic(err)
	}
	dc.cache.Store(key.String(), buf.Bytes())
}

func (dc *JsonTlbDiskCache[K, V]) Load(key K) (V, bool) {
	var value V
	data, ok := dc.cache.Load(key.String())
	if !ok {
		return value, false
	}
	cell := boc.NewCell()
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(cell); err != nil {
		panic(err)
	}
	if err := tlb.Unmarshal(cell, &value); err != nil {
		panic(err)
	}
	return value, true
}

type CachedExecResult struct {
	Code  uint32
	Stack tlb.VmStack
}

type wrappedString struct {
	value string
}

func (w wrappedString) String() string {
	return w.value
}

type CachedExecutor struct {
	executor abi.Executor
	cache    *JsonTlbDiskCache[wrappedString, CachedExecResult]
}

func (ce *CachedExecutor) RunSmcMethodByID(ctx context.Context, accountID ton.AccountID, methodID int, params tlb.VmStack) (uint32, tlb.VmStack, error) {
	argHash := sha256.New()
	argHash.Write([]byte(accountID.String()))
	argHash.Write([]byte(strconv.Itoa(methodID)))
	paramsBytes, err := params.MarshalTL()
	if err != nil {
		return 0, tlb.VmStack{}, err
	}
	argHash.Write(paramsBytes)
	argHashStr := hex.EncodeToString(argHash.Sum(nil))
	result, found := ce.cache.Load(wrappedString{argHashStr})
	if found {
		return result.Code, result.Stack, nil
	}
	code, stack, err := ce.executor.RunSmcMethodByID(ctx, accountID, methodID, params)
	if err != nil {
		return code, stack, err
	}
	ce.cache.Store(wrappedString{argHashStr}, CachedExecResult{Code: code, Stack: stack})
	return code, stack, err
}

type Tuple2[V1, V2 any] struct {
	V1 V1
	V2 V2
}
