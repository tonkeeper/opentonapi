package api

import "testing"

func Test_convertJettonDecimals(t *testing.T) {
	tests := []struct {
		decimals string
		want     int
	}{
		{decimals: "", want: 9},
		{decimals: "abc", want: 9},
		{decimals: "0", want: 0},
		{decimals: "6", want: 6},
		{decimals: "255", want: 255},
		{decimals: "256", want: 9},
		{decimals: "-1", want: 9},
		{decimals: "-9223372036854775808", want: 9},
		{decimals: "99999999999999999999", want: 9},
	}
	for _, tt := range tests {
		t.Run(tt.decimals, func(t *testing.T) {
			if got := convertJettonDecimals(tt.decimals); got != tt.want {
				t.Errorf("convertJettonDecimals(%q) = %v, want %v", tt.decimals, got, tt.want)
			}
		})
	}
}
