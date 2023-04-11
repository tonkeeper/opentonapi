package api

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"strings"
)

func (h Handler) GetAllAuctions(ctx context.Context, params oas.GetAllAuctionsParams) (oas.GetAllAuctionsRes, error) {
	auctions, err := h.storage.GetAllAuctions(ctx)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	filter := ""
	if params.Tld.Value != "" {
		filter = strings.ToLower(params.Tld.Value)
	}
	var auctionsRes oas.Auctions
	for _, a := range auctions {
		if filter != "" && !strings.HasSuffix(a.Domain, filter) {
			continue
		}
		auctionsRes.Data = append(auctionsRes.Data, oas.Auction{
			Bids:   a.Bids,
			Date:   a.Date,
			Domain: a.Domain,
			Owner:  a.Owner.ToRaw(),
			Price:  a.Price,
		})
	}
	auctionsRes.Total = int64(len(auctionsRes.Data))
	return &auctionsRes, nil
}
