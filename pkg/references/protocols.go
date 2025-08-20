package references

import "github.com/tonkeeper/tongo/ton"

const (
	Ethena = "Ethena"
	Bidask = "Bidask"
)

var (
	EthenaImage = "https://ethena.fi/shared/usde.png"
	BidaskImage = "https://bidask.finance/assets/landing/bidask-logo.webp"
)

var (
	EthenaPool = ton.MustParseAccountID("0:a11ae0f5bb47bb2945871f915a621ff281c2d786c746da74873d71d6f2aaa7a5")
)
