package api

import (
	"math/big"
	"strconv"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"
)

const UnknownJettonName = "UKWN"

// NormalizedMetadata is a special version of jetton metadata ready to be shown to the users.
// It contains a mix of information from two sources:
// 1. a jetton master metadata taken from the blockchain (onchain/offchain)
// 2. a jetton description taken from the community git https://github.com/tonkeeper/ton-assets.
// It additionally rewrites some fields if necessary.
type NormalizedMetadata struct {
	Name                string
	Description         string
	Image               string
	Symbol              string
	Decimals            int
	Verification        core.TrustType
	Social              []string
	Websites            []string
	CustomPayloadApiUri string
}

func NormalizeMetadata(meta tep64.Metadata, info *addressbook.KnownJetton, trust core.TrustType) NormalizedMetadata {
	symbol := meta.Symbol
	if symbol == "" {
		symbol = UnknownJettonName
	}
	name := meta.Name
	if name == "" {
		name = "Unknown Token"
	}
	image := references.Placeholder
	if meta.Image != "" {
		image = meta.Image
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
		trust = core.TrustWhitelist
	}

	image = imgGenerator.DefaultGenerator.GenerateImageUrl(image, 200, 200)

	return NormalizedMetadata{
		Name:                name,
		Description:         description,
		Image:               image,
		Symbol:              symbol,
		Decimals:            convertJettonDecimals(meta.Decimals),
		Verification:        trust,
		Social:              social,
		Websites:            websites,
		CustomPayloadApiUri: meta.CustomPayloadAPIURL,
	}
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

// ScaleJettons returns a proper decimal representation of jettons taking metadata.Decimals into account.
func ScaleJettons(amount big.Int, decimals int) decimal.Decimal {
	return decimal.NewFromBigInt(&amount, int32(-decimals))
}
