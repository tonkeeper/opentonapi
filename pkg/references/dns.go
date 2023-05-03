package references

import "github.com/tonkeeper/tongo"

var RootDotTon = tongo.MustParseAccountID("0:b774d95eb20543f186c06b371ab88ad704f7e256130caf96189368a7d0cb6ccf")

var DomainSuffixes = map[tongo.AccountID]string{
	RootDotTon:   ".ton",
	RootTelegram: "", //telegram use full domain
	tongo.MustParseAccountID("0:d9255340783403c635169d00aaaaaf2ab85fbb5d32c707b39a42157b7c347440"): ".dolboeb.t.me",
}
