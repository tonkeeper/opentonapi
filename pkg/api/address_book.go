package api

import "github.com/tonkeeper/opentonapi/pkg/addressbook"

// addressBook provides methods to request additional information about accounts, NFT collections and jettons
// The information is stored in "https://github.com/tonkeeper/ton-assets/" and
// is being maintained by the tonkeeper team and the community.
type addressBook interface {
	GetAddressInfoByAddress(rawAddr string) (addressbook.KnownAddress, bool)
	GetCollectionInfoByAddress(rawAddr string) (addressbook.KnownCollection, bool)
	GetJettonInfoByAddress(rawAddr string) (addressbook.KnownJetton, bool)
}
