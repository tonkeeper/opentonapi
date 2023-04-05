package api

import (
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"strconv"
	"strings"
)

func convertJettonDecimals(decimals string) int {
	if decimals == "" {
		return 9
	}
	dec, err := strconv.Atoi(decimals)
	if err != nil {
		return 9
	}
	return dec
}

func jettonPreview(addressBook addressBook, master tongo.AccountID, meta tongo.JettonMetadata, imgGenerator previewGenerator) oas.JettonPreview {
	verification := oas.JettonVerificationTypeNone
	if meta.Name == "" {
		meta.Name = "Unknown Token"
	}
	if meta.Symbol == "" {
		meta.Symbol = "UKWN"
	}
	normalizedSymbol := strings.TrimSpace(strings.ToUpper(meta.Symbol))
	if normalizedSymbol == "TON" || normalizedSymbol == "TОN" { //eng and russian
		meta.Symbol = "SCAM"
	}

	if meta.Image == "" {
		meta.Image = "https://ton.ams3.digitaloceanspaces.com/token-placeholder-288.png"
	}
	info, ok := addressBook.GetJettonInfoByAddress(master)
	if ok {
		meta.Name = rewriteIfNotEmpty(meta.Name, info.Name)
		meta.Description = rewriteIfNotEmpty(meta.Description, info.Description)
		meta.Image = rewriteIfNotEmpty(meta.Image, info.Image)
		meta.Symbol = rewriteIfNotEmpty(meta.Symbol, info.Symbol)
		verification = oas.JettonVerificationTypeWhitelist
	}

	jetton := oas.JettonPreview{
		Address:      master.ToRaw(),
		Name:         meta.Name,
		Symbol:       meta.Symbol,
		Verification: verification,
		Decimals:     convertJettonDecimals(meta.Decimals),
		Image:        imgGenerator.GenerateImageUrl(meta.Image, 200, 200),
	}
	return jetton
}
