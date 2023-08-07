package litestorage

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

func (s *LiteStorage) GetDomainInfo(ctx context.Context, domain string) (core.NftItem, int64, error) {
	return core.NftItem{}, 0, nil
}
