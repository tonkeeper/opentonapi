package litestorage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

type Cache[K, V any] interface {
	Store(key K, value V)
	Load(key K) (value V, ok bool)
}

// FsCache is a filesystem-backed key/value cache with [string, []byte] types.
// On creation it loads all existing files from the billy filesystem into memory.
// Subsequent reads are served from memory; writes go to both memory and disk.
type FsCache struct {
	mu   sync.RWMutex
	fs   billy.Filesystem
	data map[string][]byte
}

// NewFsCache creates a FsCache rooted at the given directory.
// All existing files are read into memory during construction.
func NewFsCache(dir string) (*FsCache, error) {
	c := &FsCache{
		fs:   osfs.New(dir),
		data: make(map[string][]byte),
	}
	if err := c.loadAll("/"); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *FsCache) loadAll(dir string) error {
	infos, err := c.fs.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, info := range infos {
		path := filepath.Join(dir, info.Name())
		if info.IsDir() {
			if err := c.loadAll(path); err != nil {
				return err
			}
			continue
		}
		f, err := c.fs.Open(path)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(f)
		f.Close()
		if readErr != nil {
			return readErr
		}
		// Strip the leading "/" so keys match what Store uses.
		key := path
		if len(key) > 0 && key[0] == '/' {
			key = key[1:]
		}
		c.data[key] = data
	}
	return nil
}

func (c *FsCache) Store(key string, value []byte) {
	dir := filepath.Dir(key)
	if dir != "" && dir != "." {
		if err := c.fs.MkdirAll(dir, 0755); err != nil {
			panic(fmt.Sprintf("FsCache: MkdirAll %s: %v", dir, err))
		}
	}
	f, err := c.fs.Create(key)
	if err != nil {
		panic(fmt.Sprintf("FsCache: Create %s: %v", key, err))
	}
	_, writeErr := f.Write(value)
	if closeErr := f.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		panic(fmt.Sprintf("FsCache: Write %s: %v", key, writeErr))
	}
	c.mu.Lock()
	c.data[key] = value
	c.mu.Unlock()
}

func (c *FsCache) Load(key string) ([]byte, bool) {
	c.mu.RLock()
	v, ok := c.data[key]
	c.mu.RUnlock()
	return v, ok
}

// BytesDiskCache is a Cache[K, []byte] backed by an FsCache.
type BytesDiskCache[K fmt.Stringer] struct {
	cache *FsCache
}

func (dc *BytesDiskCache[K]) Store(key K, value []byte) {
	dc.cache.Store(key.String(), value)
}

func (dc *BytesDiskCache[K]) Load(key K) ([]byte, bool) {
	return dc.cache.Load(key.String())
}

// JsonTlbDiskCache is a Cache[K, V] that serialises values as JSON-encoded TLB cells,
// backed by an FsCache.
type JsonTlbDiskCache[K fmt.Stringer, V any] struct {
	cache *FsCache
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
	argHashStr = argHashStr[:2] + "/" + argHashStr[2:]
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
