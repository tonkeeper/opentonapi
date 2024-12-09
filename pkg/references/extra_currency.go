package references

type ExtraCurrencyMeta struct {
	Name     string
	Decimals int
}

var ExtraCurrencies = map[int32]ExtraCurrencyMeta{
	239: {
		Name:     "FMS",
		Decimals: 5,
	},
}
