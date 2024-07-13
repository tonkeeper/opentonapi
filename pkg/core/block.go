package core

import (
	"github.com/tonkeeper/tongo"
)

// BlockHeader contains information extracted from a block.
type BlockHeader struct {
	tongo.BlockIDExt
	MasterRef              *tongo.BlockIDExt
	PrevBlocks             []tongo.BlockIDExt
	StartLt                int64
	EndLt                  int64
	GlobalId               int32
	MinRefMcSeqno          int32
	CatchainSeqno          int32
	PrevKeyBlockSeqno      int32
	ValidatorListHashShort int32
	GenUtime               uint32
	Version                uint32
	VertSeqno              uint32
	WantMerge              bool
	WantSplit              bool
	AfterMerge             bool
	AfterSplit             bool
	BeforeSplit            bool
	IsKeyBlock             bool
	// GenSoftware describes software that created this particular block.
	// It is up to the software to include this piece of information.
	GenSoftware *GenSoftware
	BlockExtra  BlockExtra
	ValueFlow   ValueFlow
	// TxQuantity is the number of transactions in this block.
	TxQuantity int
}

type ReducedBlock struct {
	tongo.BlockIDExt
	MasterRef    *tongo.BlockID
	Utime        int64
	ParentBlocks []tongo.BlockID
	ShardsBlocks []tongo.BlockID
	TxQuantity   int
}

// GenSoftware describes version and capabilities of software that created a blockchain block.
type GenSoftware struct {
	Version      uint32
	Capabilities uint64
}

type BlockExtra struct {
	RandSeed  tongo.Bits256
	CreatedBy tongo.Bits256
	// InMsgDescrLength is a length of the inbound message queue of a block.
	InMsgDescrLength int
	// OutMsgDescrLength is a length of the outbound message queue of a block.
	OutMsgDescrLength int
}

type Currency struct {
	ID    int64
	Value string
}

type CurrencyCollection struct {
	Grams uint64
	Other []Currency
}

type ValueFlow struct {
	FromPrevBlk   CurrencyCollection
	ToNextBlk     CurrencyCollection
	Imported      CurrencyCollection
	Exported      CurrencyCollection
	FeesCollected CurrencyCollection
	Burned        *CurrencyCollection
	FeesImported  CurrencyCollection
	Recovered     CurrencyCollection
	Created       CurrencyCollection
	Minted        CurrencyCollection
}
