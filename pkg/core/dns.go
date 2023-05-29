package core

type DnsExpiring struct {
	ExpiringAt int64
	Name       string
	DnsItem    *NftItem
}
