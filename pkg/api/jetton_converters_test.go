package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"
)

func TestJettonPreview_BlacklistedHasNoImage(t *testing.T) {
	master := ton.MustParseAccountID("EQCynJJ1RdWNXJ9vMDOF0hofz352DQ796mbvgeDDb0xcIW-S")
	meta := NormalizedMetadata{
		Name:         "Tether",
		Symbol:       "TON-USDT",
		Verification: core.TrustBlacklist,
		PreviewImage: "https://example.com/image.png",
	}

	preview := jettonPreview(master, meta, 0, nil)
	require.Empty(t, preview.Image)

	metadata := jettonMetadata(master, meta)
	require.False(t, metadata.Image.IsSet())
}

func TestJettonPreview_NonBlacklistedKeepsImage(t *testing.T) {
	master := ton.MustParseAccountID("EQCynJJ1RdWNXJ9vMDOF0hofz352DQ796mbvgeDDb0xcIW-S")
	meta := NormalizedMetadata{
		Name:         "Notcoin",
		Symbol:       "NOT",
		Verification: core.TrustNone,
		PreviewImage: "https://example.com/image.png",
		Image:        "https://example.com/image.png",
	}

	preview := jettonPreview(master, meta, 0, nil)
	require.Equal(t, meta.PreviewImage, preview.Image)

	metadata := jettonMetadata(master, meta)
	require.True(t, metadata.Image.IsSet())
	require.Equal(t, meta.Image, metadata.Image.Value)
}
