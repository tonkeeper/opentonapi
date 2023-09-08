package litestorage

import (
	"context"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
)

func (s *LiteStorage) GetLibraries(ctx context.Context, libraries []tongo.Bits256) (map[tongo.Bits256]*boc.Cell, error) {
	if len(libraries) == 0 {
		return nil, nil
	}
	libs := make(map[tongo.Bits256]*boc.Cell, len(libraries))
	var cacheMissed []tongo.Bits256
	for _, hash := range libraries {
		if cell, ok := s.tvmLibraryCache.Get(hash.Hex()); ok {
			libs[hash] = &cell
			continue
		}
		cacheMissed = append(cacheMissed, hash)
	}
	if len(cacheMissed) == 0 {
		return libs, nil
	}
	fetchedLibs, err := s.client.GetLibraries(ctx, cacheMissed)
	if err != nil {
		return nil, err
	}
	for hash, cell := range fetchedLibs {
		s.tvmLibraryCache.Set(hash.Hex(), *cell)
		libs[hash] = cell
	}
	return libs, nil
}
