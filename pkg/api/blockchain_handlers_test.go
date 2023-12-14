package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	pkgTesting "github.com/tonkeeper/opentonapi/pkg/testing"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

func TestHandler_GetRawBlockchainConfig(t *testing.T) {
	logger := zap.L()
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)
	liteStorage, err := litestorage.NewLiteStorage(logger, cli)
	require.Nil(t, err)
	h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
	require.Nil(t, err)
	cfg, err := h.GetRawBlockchainConfig(context.Background())
	require.Nil(t, err)
	require.True(t, len(cfg.Config) > 10)
}

func TestHandler_GetRawBlockchainConfigFromBlock(t *testing.T) {
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
				MasterchainSeqno: 4645003,
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
				MasterchainSeqno: 4645004,
			},
			wantErr:           "block must be a key block",
			wantErrStatusCode: 404,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.L()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
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

func TestHandler_GetBlockchainConfigFromBlock(t *testing.T) {
	tests := []struct {
		name              string
		params            oas.GetBlockchainConfigFromBlockParams
		wantErr           string
		wantErrStatusCode int
	}{
		{
			name: "all good",
			params: oas.GetBlockchainConfigFromBlockParams{
				MasterchainSeqno: 34281411,
			},
		},
		{
			name: "not a key block",
			params: oas.GetBlockchainConfigFromBlockParams{
				MasterchainSeqno: 34281410,
			},
			wantErr:           "block must be a key block",
			wantErrStatusCode: 404,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.L()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			_, err = h.GetBlockchainConfigFromBlock(context.Background(), tt.params)
			if len(tt.wantErr) > 0 {
				var oasErr *oas.ErrorStatusCode
				errors.As(err, &oasErr)
				require.Equal(t, oasErr.StatusCode, tt.wantErrStatusCode)
				require.Equal(t, oasErr.Response.Error, tt.wantErr)
				return
			}
			require.Nil(t, err)
		})
	}
}

func TestHandler_GetBlockchainValidators(t *testing.T) {
	logger := zap.L()
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)
	liteStorage, err := litestorage.NewLiteStorage(logger, cli)
	require.Nil(t, err)
	h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
	require.Nil(t, err)
	validators, err := h.GetBlockchainValidators(context.Background())
	require.Nil(t, err)

	rawConfig, err := h.storage.GetLastConfig(context.Background())
	require.Nil(t, err)
	config, err := ton.ConvertBlockchainConfig(rawConfig)
	require.Nil(t, err)

	require.NotNil(t, config.ConfigParam34)
	curValidators := config.ConfigParam34.CurValidators.ValidatorsExt
	require.Equal(t, len(validators.Validators), len(curValidators.List.Items()))
	inCurrentSet := make(map[string]struct{})
	for _, item := range curValidators.List.Items() {
		inCurrentSet[item.Value.ValidatorAddr.AdnlAddr.Hex()] = struct{}{}
	}
	for _, v := range validators.Validators {
		_, ok := inCurrentSet[v.AdnlAddress]
		require.True(t, ok)
	}
	require.Equal(t, validators.ElectAt, int64(curValidators.UtimeSince))
}

func TestHandler_GetBlockchainBlock(t *testing.T) {
	tests := []struct {
		name           string
		blockID        string
		filenamePrefix string
	}{
		{
			name:           "block from masterchain, no burned",
			blockID:        "(-1,8000000000000000,34336028)",
			filenamePrefix: "block-1-8000000000000000-34336028",
		},
		{
			name:           "block from masterchain with value burned",
			blockID:        "(-1,8000000000000000,34336196)",
			filenamePrefix: "block-1-8000000000000000-34336196",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.L()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			params := oas.GetBlockchainBlockParams{BlockID: tt.blockID}
			block, err := h.GetBlockchainBlock(context.Background(), params)
			require.Nil(t, err)
			pkgTesting.CompareResults(t, block, tt.filenamePrefix)
		})
	}
}
