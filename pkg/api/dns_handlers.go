package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"

	"github.com/tonkeeper/tongo/contract/dns"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
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
	var primaryNames, names []string
	dnsResolver, err := h.dnsResolver(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for _, d := range domains {
		records, err := dnsResolver.Resolve(ctx, d)
		if err != nil { //todo: check error type
			continue
		}
		if found, isPrimary := findAddress(records, account); found {
			if isPrimary {
				primaryNames = append(primaryNames, d)
			} else {
				names = append(names, d)
			}
		}
	}
	if len(primaryNames) > 0 {
		names = append(primaryNames, names...)
	}
	return &oas.DomainNames{Domains: names}, nil
}

func (h *Handler) DnsResolve(ctx context.Context, params oas.DnsResolveParams) (*oas.DnsRecord, error) {
	if len(params.DomainName) == 48 || len(params.DomainName) == 52 || (len(params.DomainName) == 46 && params.DomainName[:2] == "0x") {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("domains with length 46, 48 and 52 can't be resolved by security issues"))
	}
	if len(params.DomainName) > 127 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("domain name is too long"))
	}
	if regexp.MustCompile(`^sei1[a-z0-9]{38}.*`).MatchString(params.DomainName) {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("domain is not allowed"))
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
	if r, ok := records[dns.DNSCategoryDNSNextResolver]; ok && r.SumType == "DNSNextResolver" {
		result.NextResolver.SetTo(convertMsgAddress(r.DNSNextResolver))
	}
	if r, ok := records[dns.DNSCategoryWallet]; ok && r.SumType == "DNSSmcAddress" {
		result.Wallet.SetTo(mapSmcAddressToWalletDNS(r.DNSSmcAddress.Address, r.DNSSmcAddress.SmcCapability, h.addressBook))
	}
	if r, ok := records[dns.DNSCategorySite]; ok && r.SumType == "DNSAdnlAddress" {
		for _, proto := range r.DNSAdnlAddress.ProtoList {
			path := fmt.Sprintf("%v://%x", proto, r.DNSAdnlAddress.Address)
			result.Sites = append(result.Sites, path)
		}
	}
	if r, ok := records[dns.DNSCategoryStorage]; ok && r.SumType == "DNSStorageAddress" {
		result.Storage.SetTo(r.DNSStorageAddress.Hex())
	}
	if r, ok := records[dns.DNSCategoryPicture]; ok {
		if picture, ok1 := mapDNSRecordToPicture(r); ok1 {
			result.Picture.SetTo(picture)
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
	nftScamData, err := h.spamFilter.GetNftsScamData(ctx, []ton.AccountID{nft.Address})
	if err != nil {
		h.logger.Warn("error getting nft scam data", zap.Error(err))
	}
	convertedDomainInfo.Item.SetTo(h.convertNFT(ctx, nft, h.addressBook, h.metaCache, nftScamData[nft.Address]))
	if expTime != 0 {
		convertedDomainInfo.ExpiringAt.SetTo(expTime)
	}
	return &convertedDomainInfo, nil
}
