package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/tongo/contract/dns"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

// dnsResolver is a lazy initialization of DNS resolver.
// Once initialized, it always returns the same cached instance.
func (h *Handler) dnsResolver(ctx context.Context) (*dns.DNS, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dns != nil {
		return h.dns, nil
	}
	config, err := h.storage.GetLastConfig(ctx)
	if err != nil {
		return nil, err
	}
	root, ok := config.DnsRootAddr()
	if !ok {
		return nil, fmt.Errorf("no dns root in config")
	}
	h.dns = dns.NewDNS(root, h.executor)
	return h.dns, nil
}

func (h *Handler) AccountDnsBackResolve(ctx context.Context, params oas.AccountDnsBackResolveParams) (*oas.DomainNames, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	domains, err := h.storage.FindAllDomainsResolvedToAddress(ctx, account.ID, references.DomainSuffixes)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.DomainNames{Domains: []string{}}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var result []string
	dnsResolver, err := h.dnsResolver(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for _, d := range domains {
		records, err := dnsResolver.Resolve(ctx, d)
		if err != nil { //todo: check error type
			continue
		}
		found := false
		for _, r := range records {
			if r.SumType != "DNSSmcAddress" {
				continue
			}
			w, err := tongo.AccountIDFromTlb(r.DNSSmcAddress.Address)
			if err != nil || w == nil {
				break
			}
			if *w != account.ID {
				break
			}
			found = true
			break
		}
		if found {
			result = append(result, d)
		}
	}
	return &oas.DomainNames{Domains: result}, nil
}

func (h *Handler) DnsResolve(ctx context.Context, params oas.DnsResolveParams) (*oas.DnsRecord, error) {
	if len(params.DomainName) == 48 || len(params.DomainName) == 52 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("domains with length 48 and 52 can't be resolved by security issues"))
	}
	if len(params.DomainName) > 127 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("domain name is too long"))
	}
	dnsResolver, err := h.dnsResolver(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	records, err := dnsResolver.Resolve(ctx, params.DomainName)
	if errors.Is(err, dns.ErrNotResolved) || errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result := oas.DnsRecord{}
	for _, r := range records {
		switch r.SumType {
		case "DNSNextResolver":
			result.NextResolver.SetTo(convertMsgAddress(r.DNSNextResolver))
		case "DNSSmcAddress":
			w := oas.WalletDNS{
				Address: convertMsgAddress(r.DNSSmcAddress.Address),
				Names:   r.DNSSmcAddress.SmcCapability.Name,
			}
			w.Account = convertAccountAddress(ton.MustParseAccountID(w.Address), h.addressBook)
			for _, c := range r.DNSSmcAddress.SmcCapability.Interfaces {
				switch c {
				case "seqno":
					w.HasMethodSeqno = true
				case "pubkey":
					w.HasMethodPubkey = true
				case "wallet":
					w.IsWallet = true
				}
			}
			result.Wallet.SetTo(w)
		case "DNSAdnlAddress":
			for _, proto := range r.DNSAdnlAddress.ProtoList {
				path := fmt.Sprintf("%v://%x", proto, r.DNSAdnlAddress.Address)
				result.Sites = append(result.Sites, path)
			}
		case "DNSStorageAddress":
			result.Storage.SetTo(r.DNSStorageAddress.Hex())
		}
	}
	return &result, nil
}

func (h *Handler) GetDnsInfo(ctx context.Context, params oas.GetDnsInfoParams) (*oas.DomainInfo, error) {
	name, err := convertDomainName(params.DomainName)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	nft, expTime, err := h.storage.GetDomainInfo(ctx, name)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	convertedDomainInfo := oas.DomainInfo{
		Name: params.DomainName,
	}
	nftTrustType := h.convertNftTrustType(nft.CollectionAddress)
	convertedDomainInfo.Item.SetTo(convertNFT(ctx, nft, h.addressBook, h.metaCache, nftTrustType))
	if expTime != 0 {
		convertedDomainInfo.ExpiringAt.SetTo(expTime)
	}
	return &convertedDomainInfo, nil
}

func convertMsgAddress(address tlb.MsgAddress) string {
	a, _ := tongo.AccountIDFromTlb(address)
	if a == nil {
		return ""
	}
	return a.ToRaw()
}

func convertDomainName(s string) (string, error) {
	s = strings.ToLower(s)
	name := strings.TrimSuffix(s, ".ton")
	if len(name) < 4 || len(name) > 126 {
		return "", fmt.Errorf("invalid domain len")
	}
	return name, nil
}
