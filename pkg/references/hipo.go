package references

import "github.com/tonkeeper/tongo/ton"

const HipoImplementationsName = "Hipo"
const HipoImplementationsURL = "https://hipo.finance/"

var (
	// HipoTreasury is Hipo's liquid-staking pool. It holds the staked GRAM, lends it to
	// validators once per validation round and owns the hGRAM exchange rate. Its address
	// is the stable anchor of every Hipo trace: the code is upgradable in place, so the
	// treasury address never changes, while HipoParent can be replaced by an upgrade
	// (superseded parents are remembered by the treasury in its `old_parents` set).
	HipoTreasury = ton.MustParseAccountID("0:8bc991cfe177bc7e9721433efa3befd199485a55cffd040a06c89af026b71bcf")
	// HipoParent is the jetton master of hGRAM (formerly hTON). It proxies every message
	// between the treasury and a staker's hGRAM wallet.
	HipoParent = ton.MustParseAccountID("0:cf76af318c0872b58a9f1925fc29c156211782b9fb01f56760d292e56123bf87")
)
