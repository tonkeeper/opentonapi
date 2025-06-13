package litestorage

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

func (s *LiteStorage) GetAccountInvoicesHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT *int64) ([]core.InvoicePayment, error) {
	return nil, nil
}

func (s *LiteStorage) GetInvoice(ctx context.Context, source, destination tongo.AccountID, invoiceID uuid.UUID, currency string) (core.InvoicePayment, error) {
	return core.InvoicePayment{}, errors.New("not implemented yet")
}
