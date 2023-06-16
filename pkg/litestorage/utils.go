package litestorage

import (
	"fmt"
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

func hashInMsgCreatedLT(seed maphash.Seed, alt inMsgCreatedLT) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.WriteString(fmt.Sprintf("%s:%d", alt.account.String(), alt.lt))
	return h.Sum64()
}
