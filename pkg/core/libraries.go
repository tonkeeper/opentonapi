package core

import (
	"context"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	tcode "github.com/tonkeeper/tongo/code"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
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

var dynamicLibs = map[ton.Bits256]string{
	ton.MustParseHash("e9aa0c02aafd5b38a295cc489019882439cf35c9738cc6dbeece4403dd066a5a"): "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",
}

func PrepareLibraries(ctx context.Context, code *boc.Cell, accountLibraries map[tongo.Bits256]*SimpleLib, resolver LibraryResolver) (string, error) {
	if code == nil {
		return "", nil
	}
	hashes, err := tcode.FindLibraries(code)
	if err != nil {
		return "", err
	}
	if v, _ := code.Hash256(); dynamicLibs[v] != "" {
		return dynamicLibs[v], nil
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
	base64libs, err := tcode.LibrariesToBase64(libs)
	if err != nil {
		return "", err
	}
	return base64libs, nil
}
