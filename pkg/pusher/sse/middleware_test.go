package sse

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/pusher/errors"
)

func Test_writeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "bad request",
			err:      errors.BadRequest("test"),
			wantCode: 400,
		},
		{
			name:     "internal server error",
			err:      fmt.Errorf("some-err"),
			wantCode: 500,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeError(rec, tt.err)
			require.Equal(t, tt.wantCode, rec.Code)
		})
	}
}
