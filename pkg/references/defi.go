package references

import "github.com/tonkeeper/tongo/ton"

type DefiProviderMeta struct {
	Name  string
	URL   string
	Image string
}

type LiquidStakingJetton struct {
	DefiProviderMeta
	JettonMaster ton.AccountID
}

var LiquidStakingJettons = []LiquidStakingJetton{
	{
		DefiProviderMeta: DefiProviderMeta{
			Name:  "Tonstakers",
			URL:   "https://app.tonstakers.com/",
			Image: "https://cache.tonapi.io/imgproxy/qkMSC0tZBAgHZkpLfCvhQJMXnHVpFKjhGM4eMD7HWWQ/rs:fill:200:200:1/g:no/aHR0cHM6Ly90b25zdGFrZXJzLmNvbS9sb2dvLnBuZw.webp",
		},
		JettonMaster: ton.MustParseAccountID("0:bdf3fa8098d129b54b4f73b5bac5d1e1fd91eb054169c3916dfc8ccd536d1000"),
	},
	{
		DefiProviderMeta: DefiProviderMeta{
			Name:  "Stakee",
			URL:   "https://app.stakee.org/",
			Image: "https://app.stakee.org/favicon.ico",
		},
		JettonMaster: ton.MustParseAccountID("0:aa0ba121449feda569e02b12fa755d24e834a7454aecf4649590b6df742aac8f"),
	},
	{
		DefiProviderMeta: DefiProviderMeta{
			Name:  "Hipo",
			URL:   "https://app.hipo.finance/",
			Image: "https://app.hipo.finance/favicon.ico",
		},
		JettonMaster: ton.MustParseAccountID("0:cf76af318c0872b58a9f1925fc29c156211782b9fb01f56760d292e56123bf87"),
	},
	{
		DefiProviderMeta: DefiProviderMeta{
			Name:  "Bemo",
			URL:   "https://bemo.finance/",
			Image: "https://bemo.finance/favicon.ico",
		},
		JettonMaster: ton.MustParseAccountID("0:92c4664f1ea6b74ed9ce0e031a9fc0843348dfe87a58faea27fcd31e1608caaa"),
	},
	{
		DefiProviderMeta: DefiProviderMeta{
			Name:  "Storm",
			URL:   "https://storm.tg/",
			Image: "https://storm.tg/favicon.ico",
		},
		JettonMaster: ton.MustParseAccountID("0:6ca2c99c66b0fa1478a303ba9618bc39c28fda1fc50de37e618bddf98c9fd24c"),
	},
	{
		DefiProviderMeta: DefiProviderMeta{
			Name:  "UTonics",
			URL:   "https://utonic.org/home",
			Image: "https://utonic.org/favicon.ico",
		},
		JettonMaster: ton.MustParseAccountID("0:1f1798f724c2296652e6002bfb51bed11fb5a689532e5788af7203581ef367a8"),
	},
	{
		DefiProviderMeta: JVaultProvider,
		JettonMaster:     JVaultJettonMaster,
	},
}

var LiquidStakingJettonsByMaster = func() map[ton.AccountID]LiquidStakingJetton {
	m := make(map[ton.AccountID]LiquidStakingJetton, len(LiquidStakingJettons))
	for _, p := range LiquidStakingJettons {
		m[p.JettonMaster] = p
	}
	return m
}()

var (
	DedustProvider = DefiProviderMeta{
		Name:  "DeDust",
		URL:   "https://dedust.io/",
		Image: "https://dedust.io/favicon.ico",
	}

	StonfiProvider = DefiProviderMeta{
		Name:  "StonFi",
		URL:   "https://app.ston.fi/",
		Image: StonfiImage,
	}

	ToncoProvider = DefiProviderMeta{
		Name:  "Tonco",
		URL:   "https://app.tonco.io/",
		Image: ToncoImage,
	}

	WhalesProvider = DefiProviderMeta{
		Name:  "TON Whales",
		URL:   WhalesPoolImplementationsURL,
		Image: "https://tonwhales.com/favicon.ico",
	}

	TFProvider = DefiProviderMeta{
		Name:  "TON Nominators",
		URL:   TFPoolImplementationsURL,
		Image: "https://tonvalidators.org/favicon.ico",
	}

	JVaultProvider = DefiProviderMeta{
		Name:  "JVault",
		URL:   "https://jvault.xyz/",
		Image: "https://jvault.xyz/favicon.ico",
	}

	SwapCoffeeProvider = DefiProviderMeta{
		Name:  "Swap.Coffee",
		URL:   "https://swap.coffee/",
		Image: "https://swap.coffee/favicon.ico",
	}

	EvaaProvider = DefiProviderMeta{
		Name:  "Evaa",
		URL:   "https://evaa.finance/",
		Image: "https://evaa.finance/favicon.ico",
	}

	AffluentProvider = DefiProviderMeta{
		Name:  "Affluent",
		URL:   "https://affluent.finance/",
		Image: AffluentImage,
	}

	BidaskProvider = DefiProviderMeta{
		Name:  "BidAsk",
		URL:   "https://bidask.io/",
		Image: "https://bidask.io/favicon.ico",
	}
)

var EvaaMasterAddress = ton.MustParseAccountID("EQC8rUZqR_pWV1BylWUlPNBzyiTYVoBEmQkMIQDZXICfnuRr")
var JVaultJettonMaster = ton.MustParseAccountID("EQC8FoZMlBcZhZ6Pr9sHGyHzkFv9y2B5X9tN61RvucLRzFZz")
var ToncoNFTCollection = ton.MustParseAccountID("0:bffadd270a738531da7b13ba8fc403826c2586173f9ede9c316fab53bc59ac86")
var StonJettonMaster = ton.MustParseAccountID("EQA2kCVNwVsil2EM2mB0SkXytxCqQjS4mttjDpnXmwG9T6bO")
