package litestorage

import (
	"context"
	"github.com/tonkeeper/tongo/liteclient"
)

func (s *LiteStorage) GetMasterchainInfo(ctx context.Context) (liteclient.LiteServerMasterchainInfoC, error) {
	return s.client.GetMasterchainInfo(ctx)
}
