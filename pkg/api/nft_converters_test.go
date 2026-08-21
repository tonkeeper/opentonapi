package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/ton"
)

// newTestMetaCache returns a metadata cache pre-populated with the given collections, so
// getCollectionMeta never falls through to storage.
func newTestMetaCache(collections map[ton.AccountID]collectionMeta) metadataCache {
	mc := metadataCache{
		collectionsCache: cache.NewLRUCache[tongo.AccountID, collectionMeta](16, "test_nft_metadata_cache"),
	}
	for addr, meta := range collections {
		mc.collectionsCache.Set(addr, meta)
	}
	return mc
}

// TestConvertNFTTrust locks in the trust rules tonviewer used to apply client-side: an NFT with
// no collection is a scam, and an NFT inherits the trust of the collection it belongs to.
func TestConvertNFTTrust(t *testing.T) {
	nftID := ton.MustParseAccountID("EQCNmNR28mDfkwn4bwAlwJ1uhEFnjSQTZ3REz9d7IGZXU9EZ")
	collectionID := ton.MustParseAccountID("EQDaaxtmY6Dk0YzIV0zNnbUpbjZ92TJHBvO72esc0srwv8K2")

	metaCache := newTestMetaCache(map[ton.AccountID]collectionMeta{
		collectionID: {Metadata: tep64.Metadata{Name: "Some Collection"}},
	})

	tests := []struct {
		name            string
		collection      *ton.AccountID
		collectionTrust core.TrustType
		trustType       core.TrustType
		expectedTrust   oas.TrustType
	}{
		{
			name:          "NFT without a collection is a scam",
			collection:    nil,
			expectedTrust: oas.TrustType(core.TrustBlacklist),
		},
		{
			name:          "NFT in a clean collection is not a scam",
			collection:    &collectionID,
			expectedTrust: oas.TrustType(core.TrustNone),
		},
		{
			name:            "NFT inherits a blacklisted collection",
			collection:      &collectionID,
			collectionTrust: core.TrustBlacklist,
			expectedTrust:   oas.TrustType(core.TrustBlacklist),
		},
		{
			name:          "storage trust is used when the filter has no opinion",
			collection:    &collectionID,
			trustType:     core.TrustBlacklist,
			expectedTrust: oas.TrustType(core.TrustBlacklist),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				addressBook: mockAddressBook{},
				spamFilter:  mockSpamFilter{collectionTrust: tt.collectionTrust},
				metaCache:   metaCache,
			}
			item := core.NftItem{Address: nftID, CollectionAddress: tt.collection}
			got := h.convertNFT(context.Background(), item, h.addressBook, h.metaCache, tt.trustType)
			assert.Equal(t, tt.expectedTrust, got.Trust)
		})
	}
}

// TestConvertNftCollectionTrust guards the regression that motivated splitting NftTrust in two:
// convertNftCollection used to call NftTrust with a nil collection, which the no-collection rule
// would read as a scam and blacklist every collection in the API.
func TestConvertNftCollectionTrust(t *testing.T) {
	collectionID := ton.MustParseAccountID("EQDaaxtmY6Dk0YzIV0zNnbUpbjZ92TJHBvO72esc0srwv8K2")

	tests := []struct {
		name            string
		collectionTrust core.TrustType
		expectedTrust   oas.TrustType
	}{
		{
			name:          "a plain collection is not a scam",
			expectedTrust: oas.TrustType(core.TrustNone),
		},
		{
			name:            "a blacklisted collection stays blacklisted",
			collectionTrust: core.TrustBlacklist,
			expectedTrust:   oas.TrustType(core.TrustBlacklist),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				addressBook: mockAddressBook{},
				spamFilter:  mockSpamFilter{collectionTrust: tt.collectionTrust},
			}
			collection := core.NftCollection{Address: collectionID}
			got := h.convertNftCollection(collection, h.addressBook)
			assert.Equal(t, tt.expectedTrust, got.Trust)
		})
	}
}
