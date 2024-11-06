package references

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

const TFPoolImplementationsName = "TON Nominators"
const TFPoolImplementationsURL = "https://tonvalidators.org/"

// validated nominators contract code hash
var TFPoolCodeHash = tongo.MustParseHash("9A3EC14BC098F6B44064C305222CAEA2800F17DDA85EE6A8198A7095EDE10DCF")

var BemoAccount = ton.MustParseAccountID("EQDNhy-nxYFgUqzfUzImBEP67JqsyMIcyk2S5_RwNNEYku0k")

const TonstakersImplementationsName = "Tonstakers"
const LiquidImplementationsUrl = "https://tonstakers.com/"

type SlpType string

const (
	JUsdtSlpType SlpType = "jUSDT"
	TonSlpType   SlpType = "TON"
	UsdtSlpType  SlpType = "USDT"
	NotSlpType   SlpType = "NOT"
)

var SlpAccounts = map[SlpType]ton.AccountID{
	JUsdtSlpType: ton.MustParseAccountID("EQDynReiCeK8xlKRbYArpp4jyzZuF6-tYfhFM0O5ulOs5H0L"),
	TonSlpType:   ton.MustParseAccountID("EQDpJnZP89Jyxz3euDaXXFUhwCWtaOeRmiUJTi3jGYgF8fnj"),
	UsdtSlpType:  ton.MustParseAccountID("EQAz6ehNfL7_8NI7OVh1Qg46HsuC4kFpK-icfqK9J3Frd6CJ"),
	NotSlpType:   ton.MustParseAccountID("EQAG8_BzwlWkmqb9zImr9RJjjgZZCLMOQXP9PR0B1PYHvfSS"),
}

var TonstakersSocialLinks = []string{"https://t.me/thetonstakers", "https://twitter.com/tonstakers"}
var TonstakersAccountPool = ton.MustParseAccountID("EQCkWxfyhAkim3g2DjKQQg8T5P4g-Q1-K_jErGcDJZ4i-vqR")

var TFLiquidPoolCodeHash = tongo.MustParseHash("192535677eed65c20ac387efe4dd7415ad9ebb9349103e87c60e592538c9dcf3")
var TFLiquidPool = ton.MustParseAccountID("0:a45b17f28409229b78360e3290420f13e4fe20f90d7e2bf8c4ac6703259e22fa")
