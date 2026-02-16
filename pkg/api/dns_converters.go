package api

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/contract/dns"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

func mapDNSRecordToPicture(r tlb.DNSRecord) (oas.PictureDNS, bool) {
	var picture oas.PictureDNS
	switch r.SumType {
	case "DNSText":
		raw := strings.TrimSpace(string(r.DNSText))
		if isValidURL(raw) {
			picture.Type = oas.PictureDNSTypeURL
			picture.URL.SetTo(raw)
			return picture, true
		}
	case "DNSStorageAddress":
		bagID := r.DNSStorageAddress.Hex()
		if bagID != "" {
			picture.Type = oas.PictureDNSTypeBagID
			picture.BagID.SetTo(bagID)
			return picture, true
		}
	}
	return picture, false
}

func mapSmcAddressToWalletDNS(address tlb.MsgAddress, capability tlb.SmcCapabilities, book addressBook) oas.WalletDNS {
	w := oas.WalletDNS{
		Address: convertMsgAddress(address),
		Names:   capability.Name,
	}
	w.Account = convertAccountAddress(ton.MustParseAccountID(w.Address), book)
	for _, c := range capability.Interfaces {
		switch c {
		case "seqno":
			w.HasMethodSeqno = true
		case "pubkey":
			w.HasMethodPubkey = true
		case "wallet":
			w.IsWallet = true
		}
	}
	return w
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

func isValidURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	_, err := url.ParseRequestURI(raw)
	return err == nil
}

func findAddress(records map[dns.DNSCategory]tlb.DNSRecord, account ton.Address) (found bool, isPrimary bool) {
	r, ok := records[dns.DNSCategoryWallet]
	if !ok || r.SumType != "DNSSmcAddress" {
		return false, false
	}
	w, err := tongo.AccountIDFromTlb(r.DNSSmcAddress.Address)
	if err != nil || w == nil || *w != account.ID {
		return false, false
	}
	return true, slices.Contains(r.DNSSmcAddress.SmcCapability.Name, "primary")
}
