package defi

import (
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

type NormalizedJettonMeta struct {
	Name                string
	Description         string
	Image               string
	Symbol              string
	Decimals            int
	Verification        core.TrustType
	Social              []string
	Websites            []string
	CustomPayloadApiUri string
	PreviewImage        string
}

func JettonPreview(master tongo.AccountID, meta NormalizedJettonMeta, score int32) oas.JettonPreview {
	preview := oas.JettonPreview{
		Address:      master.ToRaw(),
		Name:         meta.Name,
		Symbol:       meta.Symbol,
		Verification: oas.JettonVerificationType(meta.Verification),
		Decimals:     meta.Decimals,
		Image:        meta.PreviewImage,
		Score:        score,
	}
	if meta.CustomPayloadApiUri != "" {
		preview.CustomPayloadAPIURI = oas.NewOptString(meta.CustomPayloadApiUri)
	}
	if meta.Description != "" {
		preview.SetDescription(oas.NewOptString(meta.Description))
	}
	return preview
}
