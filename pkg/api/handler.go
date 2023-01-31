package api

import (
	"context"
	"opentonapi/pkg/oas"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods
	storage                  storage
}

func NewHandler(s storage) Handler {
	return Handler{
		storage: s,
	}
}

func (h Handler) GetBlock(ctx context.Context, params oas.GetBlockParams) (r oas.GetBlockRes, _ error) {

	return &oas.Block{
		Seqno: 123,
	}, nil
}
