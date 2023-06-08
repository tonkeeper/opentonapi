package jetton

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"
)

const UnknownJettonName = "UKWN"

type VerificationType string

const (
	VerificationWhitelist VerificationType = "whitelist"
	VerificationNone      VerificationType = "none"
)

// NormalizedMetadata is a special version of jetton metadata ready to be shown to the users.
// It contains a mix of information from two sources:
// 1. a jetton master metadata taken from the blockchain (onchain/offchain)
// 2. a jetton description taken from the community git https://github.com/tonkeeper/ton-assets.
// It additionally rewrites some fields if necessary.
type NormalizedMetadata struct {
	Name         string
	Description  string
	Image        string
	Symbol       string
	Decimals     int
	Verification VerificationType
	Social       []string
	Websites     []string
}

func NormalizeMetadata(meta tep64.Metadata, info *addressbook.KnownJetton) NormalizedMetadata {
	verification := VerificationNone
	name := meta.Name
	if name == "" {
		name = "Unknown Token"
	}
	symbol := meta.Symbol
	if symbol == "" {
		symbol = UnknownJettonName
	}
	normalizedSymbol := strings.TrimSpace(strings.ToUpper(meta.Symbol))
	if normalizedSymbol == "TON" || normalizedSymbol == "TОN" { //eng and russian
		symbol = "SCAM"
	}
	image := meta.Image
	if meta.Image == "" {
		image = references.Placeholder
	}
	description := meta.Description
	var social []string
	var websites []string
	if info != nil {
		name = rewriteIfNotEmpty(name, info.Name)
		description = rewriteIfNotEmpty(description, info.Description)
		image = rewriteIfNotEmpty(image, info.Image)
		symbol = rewriteIfNotEmpty(symbol, info.Symbol)
		social = info.Social
		websites = info.Websites
		verification = VerificationWhitelist
	}

	return NormalizedMetadata{
		Name:         name,
		Description:  description,
		Image:        image,
		Symbol:       symbol,
		Decimals:     convertJettonDecimals(meta.Decimals),
		Verification: verification,
		Social:       social,
		Websites:     websites,
	}
}

func rewriteIfNotEmpty(src, dest string) string {
	if dest != "" {
		return dest
	}
	return src
}

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

// Scale returns a proper decimal representation of jettons taking metadata.Decimals into account.
func Scale(amount tlb.VarUInteger16, decimals int) decimal.Decimal {
	value := big.Int(amount)
	return decimal.NewFromBigInt(&value, int32(-decimals))
}
