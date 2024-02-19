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

var TonstakersSocialLinks = []string{"https://t.me/thetonstakers", "https://twitter.com/tonstakers"}
var TonstakersAccountPool = ton.MustParseAccountID("EQCkWxfyhAkim3g2DjKQQg8T5P4g-Q1-K_jErGcDJZ4i-vqR")

var TFLiquidPoolCodeHash = tongo.MustParseHash("192535677eed65c20ac387efe4dd7415ad9ebb9349103e87c60e592538c9dcf3")
var TFLiquidPool = ton.MustParseAccountID("0:a45b17f28409229b78360e3290420f13e4fe20f90d7e2bf8c4ac6703259e22fa")
