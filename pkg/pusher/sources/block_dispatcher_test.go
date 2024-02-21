package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/internal/g"
)

func TestBlockDispatcher_RegisterSubscriber(t *testing.T) {
	tests := []struct {
		name    string
		options []SubscribeToBlockHeadersOptions
	}{
		{
			name: "subscribe",
			options: []SubscribeToBlockHeadersOptions{
				{Workchain: g.Pointer(-1)},
				{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disp := NewBlockDispatcher(zap.L())
			var cancels []CancelFn
			for _, opts := range tt.options {
				cancelFn := disp.RegisterSubscriber(func(eventData []byte) {}, opts)
				require.NotNil(t, cancelFn)
				cancels = append(cancels, cancelFn)
			}
			require.Equal(t, len(tt.options), len(disp.subscribes))

			for _, cancel := range cancels {
				cancel()
			}
			require.Equal(t, 0, len(disp.subscribes))
		})
	}
}

func Test_createBlockDeliveryFnBasedOnOptions(t *testing.T) {
	tests := []struct {
		name       string
		options    SubscribeToBlockHeadersOptions
		workchain  int
		wantCalled bool
	}{
		{
			name:       "subscribe to all workchains",
			options:    SubscribeToBlockHeadersOptions{},
			workchain:  -1,
			wantCalled: true,
		},
		{
			name:       "subscribe to all workchains",
			options:    SubscribeToBlockHeadersOptions{},
			workchain:  0,
			wantCalled: true,
		},
		{
			name:       "subscribe to masterchain",
			options:    SubscribeToBlockHeadersOptions{Workchain: g.Pointer(-1)},
			workchain:  -1,
			wantCalled: true,
		},
		{
			name:       "subscribe to basechain",
			options:    SubscribeToBlockHeadersOptions{Workchain: g.Pointer(0)},
			workchain:  -1,
			wantCalled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			fn := createBlockDeliveryFnBasedOnOptions(func(eventData []byte) {
				called = true
			}, tt.options)
			fn([]byte{}, tt.workchain)

			require.Equal(t, tt.wantCalled, called)
		})
	}
}
