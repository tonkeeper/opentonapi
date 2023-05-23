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

func hashAccountID(seed maphash.Seed, s tongo.AccountID) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.WriteString(s.String())
	return h.Sum64()
}

func hashBits256(seed maphash.Seed, s tongo.Bits256) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.WriteString(s.Hex())
	return h.Sum64()
}
