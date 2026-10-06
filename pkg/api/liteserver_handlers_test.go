package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/tonkeeper/tongo/liteclient"
)

func TestLiteServerErrorStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "block not in db",
			err:  fmt.Errorf("wrapped: %w", liteclient.LiteServerErrorC{Code: 651, Message: "block is not in db"}),
			want: http.StatusNotFound,
		},
		{
			name: "other liteserver error",
			err:  liteclient.LiteServerErrorC{Code: 652, Message: "timeout"},
			want: http.StatusInternalServerError,
		},
		{
			name: "not a liteserver error",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := liteServerErrorStatus(tt.err); got != tt.want {
				t.Errorf("liteServerErrorStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
