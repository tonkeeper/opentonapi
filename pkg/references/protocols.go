package references

import "github.com/tonkeeper/tongo/ton"

const (
	Ethena = "Ethena"
)

var (
	EthenaImage = "https://ethena.fi/shared/usde.png"
	BidaskImage = "https://bidask.finance/assets/landing/bidask-logo.webp"
	StonfiImage = "https://static.ston.fi/favicon/android-chrome-192x192.png"
	ToncoImage  = "https://ton.app/media/1f913e65-9c32-433e-a0a3-a7c5ccf46ad5.png"
)

var (
	EthenaPool  = ton.MustParseAccountID("0:a11ae0f5bb47bb2945871f915a621ff281c2d786c746da74873d71d6f2aaa7a5")
	ToncoRouter = ton.MustParseAccountID("0:bffadd270a738531da7b13ba8fc403826c2586173f9ede9c316fab53bc59ac86")
)
