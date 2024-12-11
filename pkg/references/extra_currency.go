package references

const DefaultExtraCurrencyDecimals = 9

type ExtraCurrencyMeta struct {
	Name     string
	Symbol   string
	Image    string
	Decimals int
}

var extraCurrencies = map[int32]ExtraCurrencyMeta{
	239: {
		Name:     "FMS",
		Decimals: 5,
		Symbol:   "FMS",
	},
}

func GetExtraCurrencyMeta(id int32) ExtraCurrencyMeta {
	meta, ok := extraCurrencies[id]
	if ok {
		return meta
	}
	return ExtraCurrencyMeta{
		Decimals: DefaultExtraCurrencyDecimals,
		// TODO: add default placeholders
	}
}
