package api

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/tongo"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"
)

func TestHandler_GetStakingPoolInfo(t *testing.T) {
	tests := []struct {
		name     string
		params   oas.GetStakingPoolInfoParams
		want     *oas.GetStakingPoolInfoOK
		wantErr  bool
		wantCode int
	}{
		{
			name: "no pool",
			params: oas.GetStakingPoolInfoParams{
				AccountID: "UQBszTJahYw3lpP64ryqscKQaDGk4QpsO7RO6LYVvKHSIIlx",
			},
			wantErr:  true,
			wantCode: 404,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.L()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			book := &mockAddressBook{
				OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
					return addressbook.KnownAddress{}, false
				},
			}
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage), WithAddressBook(book))
			require.Nil(t, err)
			info, err := h.GetStakingPoolInfo(context.Background(), tt.params)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantCode, err.(*oas.ErrorStatusCode).StatusCode)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.want, info)
		})
	}
}
