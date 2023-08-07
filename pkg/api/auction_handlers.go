package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h Handler) GetAllAuctions(ctx context.Context, params oas.GetAllAuctionsParams) (*oas.Auctions, error) {
	auctions, err := h.storage.GetAllAuctions(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
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

func (h Handler) GetDomainBids(ctx context.Context, params oas.GetDomainBidsParams) (*oas.DomainBids, error) {
	domain := strings.ToLower(params.DomainName)
	bids, err := h.storage.GetDomainBids(ctx, strings.TrimSuffix(domain, ".ton"))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var domainBids oas.DomainBids
	for _, bid := range bids {
		domainBids.Data = append(domainBids.Data, oas.DomainBid{
			Bidder:  convertAccountAddress(bid.Bidder, h.addressBook, h.previewGenerator),
			Success: bid.Success,
			TxTime:  bid.TxTime,
			Value:   int64(bid.Value),
			TxHash:  bid.TxHash.Hex(),
		})
	}
	return &domainBids, nil
}
