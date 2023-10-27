package api

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/config"
	"go.uber.org/zap"
)

func TestHandler_GetRawBlockchainConfig(t *testing.T) {
	logger := zap.L()
	liteStorage, err := litestorage.NewLiteStorage(logger)
	require.Nil(t, err)
	h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
	require.Nil(t, err)
	cfg, err := h.GetRawBlockchainConfig(context.Background())
	require.Nil(t, err)
	require.True(t, len(cfg.Config) > 10)
}

func TestHandler_GetRawBlockchainConfigFromBlock(t *testing.T) {
	var servers []config.LiteServer
	if env, ok := os.LookupEnv("LITE_SERVERS"); ok {
		var err error
		servers, err = config.ParseLiteServersEnvVar(env)
		require.Nil(t, err)
	}
	tests := []struct {
		name              string
		params            oas.GetRawBlockchainConfigFromBlockParams
		wantKeys          map[string]struct{}
		wantErr           string
		wantErrStatusCode int
	}{
		{
			name: "all good",
			params: oas.GetRawBlockchainConfigFromBlockParams{
				BlockID: "(-1,8000000000000000,4645003)",
			},
			wantKeys: map[string]struct{}{
				"config_param0":  {},
				"config_param1":  {},
				"config_param10": {},
				"config_param11": {},
				"config_param12": {},
				"config_param14": {},
				"config_param15": {},
				"config_param16": {},
				"config_param17": {},
				"config_param18": {},
				"config_param2":  {},
				"config_param20": {},
				"config_param21": {},
				"config_param22": {},
				"config_param23": {},
				"config_param24": {},
				"config_param25": {},
				"config_param28": {},
				"config_param29": {},
				"config_param31": {},
				"config_param32": {},
				"config_param34": {},
				"config_param4":  {},
				"config_param7":  {},
				"config_param8":  {},
				"config_param9":  {},
			},
		},
		{
			name: "not a key block",
			params: oas.GetRawBlockchainConfigFromBlockParams{
				BlockID: "(-1,8000000000000000,4645004)",
			},
			wantErr:           "block must be a key block",
			wantErrStatusCode: 404,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.L()
			liteStorage, err := litestorage.NewLiteStorage(logger, litestorage.WithLiteServers(servers))
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			cfg, err := h.GetRawBlockchainConfigFromBlock(context.Background(), tt.params)
			if len(tt.wantErr) > 0 {
				var oasErr *oas.ErrorStatusCode
				errors.As(err, &oasErr)
				require.Equal(t, oasErr.StatusCode, tt.wantErrStatusCode)
				require.Equal(t, oasErr.Response.Error, tt.wantErr)
				return
			}
			require.Nil(t, err)
			keys := make(map[string]struct{})
			for key := range cfg.Config {
				keys[key] = struct{}{}
			}
			require.Equal(t, tt.wantKeys, keys)
		})
	}
}
