package litestorage

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

func (s *LiteStorage) GetAccountInvoicesHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT *int64) ([]core.InvoicePayment, error) {
	return nil, nil
}
