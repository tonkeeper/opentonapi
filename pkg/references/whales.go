package references

import "github.com/tonkeeper/tongo"

type WhalesPoolInfo struct {
	Name    string
	Queue   string
	Percent float64
}

var WhalesPools = map[tongo.AccountID]WhalesPoolInfo{
	tongo.MustParseAccountID("EQBYtJtQzU3M-AI23gFM91tW6kYlblVtjej59gS8P3uJ_ePN"): {
		Name:  "EPn",
		Queue: "Queue #1",
	},
	tongo.MustParseAccountID("EQBYtJtQzU3M-AI23gFM91tW6kYlblVtjej59gS8P3uJ_ePN"): {
		Name:  "EPn",
		Queue: "Queue #2",
	},
	tongo.MustParseAccountID("EQDFvnxuyA2ogNPOoEj1lu968U4PP8_FzJfrOWUsi_o1CLUB"): {
		Name:  "Whales Club",
		Queue: "Queue #1",
	},
	tongo.MustParseAccountID("EQA_cc5tIQ4haNbMVFUD1d0bNRt17S7wgWEqfP_xEaTACLUB"): {
		Name:  "Whales Club",
		Queue: "Queue #2",
	},
	tongo.MustParseAccountID("EQCkR1cGmnsE45N4K0otPl5EnxnRakmGqeJUNua5fkWhales"): {
		Name:  "Whales Nominators",
		Queue: "Queue #1",
	},
	tongo.MustParseAccountID("EQCY4M6TZYnOMnGBQlqi_nyeaIB1LeBFfGgP4uXQ1VWhales"): {
		Name:  "Whales Nominators",
		Queue: "Queue #2",
	},
	tongo.MustParseAccountID("EQCOj4wEjXUR59Kq0KeXUJouY5iAcujkmwJGsYX7qPnITEAM"): {
		Name:  "Whales Team",
		Queue: "Queue #1",
	},
	tongo.MustParseAccountID("EQBI-wGVp_x0VFEjd7m9cEUD3tJ_bnxMSp0Tb9qz757ATEAM"): {
		Name:  "Whales Team",
		Queue: "Queue #2",
	},
}
