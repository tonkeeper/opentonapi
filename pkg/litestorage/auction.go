package litestorage

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

func (s *LiteStorage) GetAllAuctions(ctx context.Context) ([]core.Auction, error) {
	return nil, nil
}

func (s *LiteStorage) GetDomainBids(ctx context.Context, domain string) ([]core.DomainBid, error) {
	return nil, nil
}
