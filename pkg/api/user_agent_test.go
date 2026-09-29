package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTonkeeperUserAgent(t *testing.T) {
	tests := []struct {
		name   string
		client client
		want   bool
	}{
		{name: "empty", want: false},
		{name: "bare name", client: client{userAgent: "Tonkeeper"}, want: true},
		{name: "product token with version", client: client{userAgent: "Tonkeeper/5.0.0 (iOS 18.0)"}, want: true},
		{name: "keeper", client: client{userAgent: "keeper"}, want: true},
		{name: "desktop", client: client{userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Tonkeeper/4.2.0 Chrome/138.0.7204.100 Electron/37.2.0 Safari/537.36"}, want: true},
		{name: "browser", client: client{userAgent: "Mozilla/5.0"}, want: false},
		{name: "wallet.tonkeeper.com origin", client: client{userAgent: "Mozilla/5.0", origin: "https://wallet.tonkeeper.com"}, want: true},
		{name: "nested subdomain origin", client: client{origin: "https://a.b.tonkeeper.com"}, want: true},
		{name: "apex origin", client: client{origin: "https://tonkeeper.com"}, want: true},
		{name: "origin with port and upper case", client: client{origin: "https://Wallet.Tonkeeper.com:443"}, want: true},
		{name: "lookalike origin", client: client{origin: "https://eviltonkeeper.com"}, want: false},
		{name: "tonkeeper.com as a prefix", client: client{origin: "https://tonkeeper.com.evil.io"}, want: false},
		{name: "opaque origin", client: client{origin: "null"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTonkeeperUserAgent(tt.client))
		})
	}
}
