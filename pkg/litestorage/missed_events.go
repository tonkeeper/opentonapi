package litestorage

import (
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/net/context"
)

func (s *LiteStorage) GetMissedEvents(ctx context.Context, account ton.AccountID, lt uint64, limit int) ([]oas.AccountEvent, error) {
	return nil, nil
}
