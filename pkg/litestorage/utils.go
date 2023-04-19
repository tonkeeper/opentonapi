package litestorage

import (
	"hash/maphash"

	"github.com/tonkeeper/tongo"
)

func hashBlockIDExt(seed maphash.Seed, s tongo.BlockIDExt) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.WriteString(s.String())
	return h.Sum64()
}
