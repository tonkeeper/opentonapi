package wallet

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/wallet"
)

func TestExtractRisk(t *testing.T) {
	tests := []struct {
		name    string
		boc     string
		want    *Risk
		wantErr string
	}{
		{
			name: "transfer ton",
			boc:  "te6ccgEBAgEAqgAB4YgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAAQ+B903cV6YIMdtd4QtdyekehadSk+QjIgoIiRgjZD9v81PVGEXBKHPgPUknVvxvr/LGcKkLNhY+I1Wuwi/7ACU1NGLsi5dhQAAAA8AAcAQBoQgApn5hvK5EKvcI4+qgdz+LABkbBy/PLofvLWI8wTW1zT6WWgvAAAAAAAAAAAAAAAAAAAA==",
			want: &Risk{
				Ton:     3_000_000_000,
				Jettons: map[tongo.AccountID]big.Int{},
				Nfts:    nil,
			},
		},
		{
			name: "transfer jettons",
			boc:  "te6ccgECAwEAAQUAAeGIANmaZLULGG8tJ/XFeVVjhSDQY0nCFNh3aJ3RbCt5Q6RABSMjS4x6Gq0Zqdbt/8u9KDhBmpjeDE1mJwmaGkKpoKmNpuFpsf2j6g/KVbw9kWLcEdc/rCcX6euh2ksWAyZx6AFNTRi7I89J2AAAASAAHAEBaGIAS1ZNypaCh7zgPRcvBcpDlS3gxPwxnEFWGfVBevyzhRwhMS0AAAAAAAAAAAAAAAAAAAECALAPin6lAAAAAAAAAAAxtgM4AKZ+YbyuRCr3COPqoHc/iwAZGwcvzy6H7y1iPME1tc0/ABszTJahYw3lpP64ryqscKQaDGk4QpsO7RO6LYVvKHSIAgIAAAAA",
			want: &Risk{
				Jettons: map[tongo.AccountID]big.Int{
					tongo.MustParseAccountID("0:96ac9b952d050f79c07a2e5e0b94872a5bc189f8633882ac33ea82f5f9670a38"): *big.NewInt(1794099),
				},
				Ton: 640_000_000,
			},
		},
		{
			name: "transfer nft",
			boc:  "te6ccgECAwEAAQAAAeGIANmaZLULGG8tJ/XFeVVjhSDQY0nCFNh3aJ3RbCt5Q6RAAR/y7WiDk/zi6/QObgK7qDZRawFY0k5TaspQuK98GHfLWcVcMgc/kdpXj+nNrmpWHO2mJ6nyxhuxwzzphZVmuBlNTRi7I88W+AAAARAAHAEBaGIAYeITnAruocV3ZaCBjfbcIK27S8GFMv5jOh6XPwNuAUkgFykzCAAAAAAAAAAAAAAAAAECAKVfzD0UAAAAAAAAAACACmfmG8rkQq9wjj6qB3P4sAGRsHL88uh+8tYjzBNbXNPwAbM0yWoWMN5aT+uK8qrHCkGgxpOEKbDu0Tui2Fbyh0iAcxLQCA==",
			want: &Risk{
				Jettons: map[tongo.AccountID]big.Int{},
				Ton:     48_572_001,
				Nfts: []tongo.AccountID{
					tongo.MustParseAccountID("0:c3c4273815dd438aeecb41031bedb8415b7697830a65fcc6743d2e7e06dc0292"),
				},
			},
		},
		{
			name: "jetton burn",
			boc:  "te6ccgEBAwEA0QAB4YgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAERYqYf/AQLsoaKRNPN7EZ6psYUfF6tePIUVmp73mKuJUnCLzm6Dg21F+C61R6vR7J4MlVKpZI+j1Z712vFW6oUU1NGLsk2BJoAAAB0AAcAQGHYgAJB2aiF7KQ/MqPG/iowHH4XVJoO7U00zTZL6YsVn8DKaHc1lAAAAAAAAAAAAAAAAAAAFlfB7x2mKNkA9fVjzGM4yMCACj1wAJckeZgvsTeh8gmpa+viwUnkQ==",
			want: &Risk{
				Jettons: map[tongo.AccountID]big.Int{
					tongo.MustParseAccountID("0:120ecd442f6521f9951e37f15180e3f0baa4d0776a69a669b25f4c58acfe0653"): *big.NewInt(1625650),
				},
				Ton: 1_000_000_000,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := tongo.ParseTlbMessage(tt.boc)
			require.Nil(t, err)
			risk, err := ExtractRisk(wallet.V4R1, msg.TlbMsg)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.want, risk)
		})
	}
}
