package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	pkgTesting "github.com/tonkeeper/opentonapi/pkg/testing"
)

func TestHandler_GetJettonsBalances(t *testing.T) {
	tests := []struct {
		name           string
		params         oas.GetJettonsBalancesParams
		filenamePrefix string
	}{
		{
			name:           "all good",
			params:         oas.GetJettonsBalancesParams{AccountID: "0:533f30de5722157b8471f5503b9fc5800c8d8397e79743f796b11e609adae69f"},
			filenamePrefix: "jetton-balances",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			liteStorage, err := litestorage.NewLiteStorage(logger, litestorage.WithKnownJettons([]tongo.AccountID{
				tongo.MustParseAccountID("0:beb5d4638e860ccf7317296e298fde5b35982f4725b0676dc98b1de987b82ebc"), // Jetton kingy
				tongo.MustParseAccountID("0:65de083a0007638233b6668354e50e44cd4225f1730d66b8b1f19e5d26690751"), // Lavandos
			}))
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			res, err := h.GetJettonsBalances(context.Background(), tt.params)
			require.Nil(t, err)
			pkgTesting.CompareResults(t, res, tt.filenamePrefix)
		})
	}
}
