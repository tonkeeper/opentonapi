package core

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/tongo/tlb"
)

// ExtraCurrency represents Other part of tlb.CurrencyCollection
// and it is a collection of currencies that are not Grams.
type ExtraCurrency map[string]string

func ToExtraCurrency(collection tlb.ExtraCurrencyCollection) ExtraCurrency {
	if len(collection.Dict.Keys()) == 0 {
		return nil
	}
	res := make(ExtraCurrency, len(collection.Dict.Keys()))
	for _, item := range collection.Dict.Items() {
		value := big.Int(item.Value)
		res[fmt.Sprintf("%d", item.Key)] = value.String()
	}
	return res
}

func (c ExtraCurrency) IsNotEmpty() bool {
	return len(c) > 0
}
