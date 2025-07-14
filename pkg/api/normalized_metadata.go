package api

import (
	"github.com/tonkeeper/tongo/ton"
	"strconv"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/tep64"
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
	PreviewImage        string // path to the converted image
}

func NormalizeMetadata(addr ton.AccountID, meta tep64.Metadata, info *addressbook.KnownJetton, trust core.TrustType) NormalizedMetadata {
	symbol := meta.Symbol
	if symbol == "" {
		symbol = UnknownJettonName + addr.ToHuman(true, false)[44:]
	}
	name := meta.Name
	if name == "" {
		name = "Unknown Token" + addr.ToHuman(true, false)[44:]
	}
	var image string
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
	previewImage := references.Placeholder
	previewImage = rewriteIfNotEmpty(previewImage, image)
	previewImage = imgGenerator.DefaultGenerator.GenerateImageUrl(previewImage, 200, 200)

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
		PreviewImage:        previewImage,
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
