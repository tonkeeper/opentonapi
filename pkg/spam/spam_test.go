package spam

import (
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"
)

func TestNftTrustBlacklistedImageHost(t *testing.T) {
	f := Filter{}
	tests := []struct {
		name  string
		image string
		want  core.TrustType
	}{
		{name: "exact blacklisted host", image: "https://cloudmetrics.cyou/image.png", want: core.TrustBlacklist},
		{name: "subdomain of blacklisted host", image: "https://cdn.cloudmetrics.cyou/a/b/image.png", want: core.TrustBlacklist},
		{name: "blacklisted host uppercase", image: "https://CloudMetrics.Cyou/image.png", want: core.TrustBlacklist},
		{name: "lookalike host is not blacklisted", image: "https://cloudmetrics.cyou.example.com/image.png", want: core.TrustNone},
		{name: "substring is not blacklisted", image: "https://notcloudmetrics.cyou/image.png", want: core.TrustNone},
		{name: "clean host", image: "https://example.com/image.png", want: core.TrustNone},
		{name: "empty image", image: "", want: core.TrustNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.NftTrust(ton.AccountID{}, nil, "name", "description", tt.image, "", "")
			if got != tt.want {
				t.Fatalf("NftTrust(image=%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}
