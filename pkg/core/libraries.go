package core

import (
	"context"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/txemulator"
)

// LibraryResolver provides a method to resolve libraries by their hashes.
type LibraryResolver interface {
	GetLibraries(ctx context.Context, libraries []tongo.Bits256) (map[tongo.Bits256]*boc.Cell, error)
}

type SimpleLib struct {
	Public bool
	Root   *boc.Cell
}

func SimpleLibMapToCells(libraries map[string]tlb.SimpleLib) map[tongo.Bits256]*SimpleLib {
	if len(libraries) == 0 {
		return nil
	}
	libs := make(map[tongo.Bits256]*SimpleLib, len(libraries))
	for libHash, lib := range libraries {
		libs[tongo.MustParseHash(libHash)] = &SimpleLib{
			Public: lib.Public,
			Root:   &lib.Root,
		}
	}
	return libs
}

func StateInitLibraries(hashmap *tlb.HashmapE[tlb.Bits256, tlb.SimpleLib]) map[tongo.Bits256]*SimpleLib {
	if hashmap == nil {
		return nil
	}
	items := hashmap.Items()
	if len(items) == 0 {
		return nil
	}
	libraries := make(map[tongo.Bits256]*SimpleLib, len(items))
	for _, item := range items {
		libraries[tongo.Bits256(item.Key)] = &SimpleLib{
			Public: item.Value.Public,
			Root:   &item.Value.Root,
		}
	}
	return libraries
}

func PrepareLibraries(ctx context.Context, code *boc.Cell, accountLibraries map[tongo.Bits256]*SimpleLib, resolver LibraryResolver) (string, error) {
	if code == nil {
		return "", nil
	}
	hashes, err := txemulator.FindLibraries(code)
	if err != nil {
		return "", err
	}
	if len(hashes) == 0 && len(accountLibraries) == 0 {
		return "", nil
	}
	libs := make(map[tongo.Bits256]*boc.Cell, len(accountLibraries))
	for hash, lib := range accountLibraries {
		libs[hash] = lib.Root
	}
	publicLibs, err := resolver.GetLibraries(ctx, hashes)
	if err != nil {
		return "", err
	}
	for hash, lib := range publicLibs {
		libs[hash] = lib
	}
	base64libs, err := txemulator.LibrariesToBase64(libs)
	if err != nil {
		return "", err
	}
	return base64libs, nil
}
