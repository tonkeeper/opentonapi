package api

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"go.uber.org/zap"

	pkgTesting "github.com/arnac-io/opentonapi/pkg/testing"
)

func Test_convertConfig(t *testing.T) {
	tests := []struct {
		name                    string
		configParamsBocFilename string
		filenamePrefix          string
	}{
		{
			name:                    "all good",
			configParamsBocFilename: "testdata/config-params-1.boc",
			filenamePrefix:          "config-params-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(tt.configParamsBocFilename)
			require.Nil(t, err)
			configCell, err := boc.DeserializeSinglRootBase64(string(content))
			require.Nil(t, err)

			var params tlb.ConfigParams
			err = tlb.Unmarshal(configCell, &params)
			require.Nil(t, err)
			got, err := convertConfig(zap.L(), params)
			require.Nil(t, err)
			pkgTesting.CompareResults(t, got, tt.filenamePrefix)
		})
	}
}
