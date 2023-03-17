package image

import (
	"github.com/tonkeeper/tongo/liteapi"
)

type PreviewGenerator struct {
	client *liteapi.Client
}

func (g *PreviewGenerator) GenerateImageUrl(url string, height, width int) string {
	return url
}
