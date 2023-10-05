package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
)

func TestPrepareLibraries(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantLibs string
	}{
		{
			name:     "code with libs",
			code:     "te6ccgEBAQEAIwAIQgLENdNBXfK6hxn//oTRKiP+O1rbrH7F/yJoMMfUoA/niA==",
			wantLibs: "te6ccgECEgEAAfYAAUOgGIa6aCu+V1DjP//QmiVEf8drW3WP2L/kTQYY+pQB/PEEAQEU/wD0pBP0vPLICwICASADBAIBSAUGAD7y7UTQ0x/TT9P/9ATRBNMfAYIQc2lnbrqSXwXhBPACAgLOBwgCASAMDQIBIAkKAIVASDCNcYINNP0x/THwL4I7vyZCe68qFRFbryogH5AVQQI/kQ8qP4AAOkIMjLH1Iwy09SIMv/UlD0AMntVPgPAwTwAYAO87aLt+zLQ0wMBcbCRW+Ah10nBIJFb4AHTHyGCEGV4dG69IoIQc2lnbr2wkl8D4O1E0NMf00/T//QE0SWCEGV4dG66jh8G+kAw+kQBpLImgwf0Dm+hMbOUXwbbMeBUcyFTOPABkTbiBIIQc2lnbrqUQDTwApJfBeKABGyUBNMAAYrobEHUMO1VgCwDq0x8hghAf+OoLupTUAe1U3iGCEBxA25+6IoIQXq70pLqxjkv6QAH6RFIQAaSyI4IQHEDbn7qeAcjKB1QgCIMH9FPypwaRMeICghBervSkupgFgwf0W/KoBJEx4iPIyx9SMMtPUiDL/1JQ9ADJ7VSRMeLUMNAEAgEgDg8AG75fD2omhAgLhrkPoCGEABm7Oc7UTQgHDXIdcL/4AgEgEBEAEbWS/aiaGuFj8AAZtF0dqJoQBBrkOuFp8A==",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, err := liteapi.NewClient(liteapi.Testnet())
			require.Nil(t, err)
			codeCell, err := boc.DeserializeSinglRootBase64(tt.code)
			require.Nil(t, err)

			libs, err := PrepareLibraries(context.Background(), codeCell, nil, cli)
			require.Nil(t, err)
			require.Equal(t, tt.wantLibs, libs)
		})
	}
}
