package spam

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

// Filter is a no-op implementation of api.SpamFilter. The real spam/scam
// detection logic lives in github.com/tonkeeper/tonapi2/pkg/spam; this stub
// only exists so opentonapi keeps compiling and running standalone (it also
// backs tonapi2's testnet code path, which intentionally avoids depending on
// the full storage-backed filter).
type Filter struct{}

func NewSpamFilter() *Filter {
	return &Filter{}
}

func (f *Filter) GetNftsScamData(ctx context.Context, addresses []ton.AccountID) (map[ton.AccountID]core.TrustType, error) {
	return nil, nil
}

func (f *Filter) GetEventsScamData(ctx context.Context, ids []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (f Filter) IsScamEvent(actions []oas.Action, viewer *ton.AccountID, initiator ton.AccountID) bool {
	return false
}

func (f Filter) JettonTrust(address tongo.AccountID, symbol, name, image string) core.TrustType {
	return core.TrustNone
}

func (f Filter) NftTrust(address tongo.AccountID, collection, owner *ton.AccountID, collectionTrust core.TrustType, name, description, image string) core.TrustType {
	return core.TrustNone
}

func (f Filter) NftCollectionTrust(address tongo.AccountID, owner *ton.AccountID, name, description, image string) core.TrustType {
	return core.TrustNone
}

func (f Filter) AccountTrust(address tongo.AccountID) core.TrustType {
	return core.TrustNone
}

func (f Filter) HasBlacklistedComment(values ...string) bool {
	return false
}

func (f Filter) TonDomainTrust(domain string) core.TrustType {
	return core.TrustNone
}
