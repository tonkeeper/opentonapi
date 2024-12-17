package references

import "fmt"

const DefaultExtraCurrencyDecimals = 9

type ExtraCurrencyMeta struct {
	Decimals int
	Symbol   string
	Image    string
}

var extraCurrencies = map[int32]ExtraCurrencyMeta{
	239: {
		Decimals: 5,
		Symbol:   "FMS",
	},
	100: {
		Decimals: 8,
		Symbol:   "ECHIDNA",
	},
}

func GetExtraCurrencyMeta(id int32) ExtraCurrencyMeta {
	res := ExtraCurrencyMeta{
		Decimals: DefaultExtraCurrencyDecimals,
		Symbol:   fmt.Sprintf("$%d", id),
		Image:    Placeholder,
	}
	meta, ok := extraCurrencies[id]
	if !ok {
		return res
	}
	if meta.Decimals > 0 {
		res.Decimals = meta.Decimals
	}
	if meta.Image != "" {
		res.Image = meta.Image
	}
	if meta.Symbol != "" {
		res.Symbol = meta.Symbol
	}
	return res
}
