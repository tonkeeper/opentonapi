package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/arnac-io/opentonapi/pkg/litestorage"
	"github.com/arnac-io/opentonapi/pkg/oas"
	pkgTesting "github.com/arnac-io/opentonapi/pkg/testing"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"
)

func TestHandler_DecodeMessage(t *testing.T) {
	tests := []struct {
		name           string
		boc            string
		filenamePrefix string
		wantErr        string
	}{
		{
			name:           "v4",
			boc:            "te6ccgECAwEAAQwAAeGIANmaZLULGG8tJ/XFeVVjhSDQY0nCFNh3aJ3RbCt5Q6RAAkDURMAlnNzb/CuopC2Ojq8jeHSH4miO3bRzGsp0U1lIecu5Fig1ZRTXEABj+/ahUiCdLf9wXtu7j27v6e/r+FlNTRi7KtlbkAAAB4gAHAEB1WIACKhajFkxNWqMTPzEQ/xBJbgDKisir7/xQJ+Ak0zSAw+hOV62UAAAAAAAAAAAAAAAAAAPin6lAAvzZd2mp+5BkqBgqADvO5kConGyoByJOKUjz+JOcYR6rramIAAe1Ep3rA5wnBA4B0MDAgBP/Pnlj4AClYDlisUiRlr5snu9gkBGuLy568QE/pN3i1RO3LpEmwSO/Q==",
			filenamePrefix: "decode-message-v4",
		},
		{
			name:           "highload",
			boc:            "te6ccgECCQEAAUMAAUWIAbeTPaOhIeFpX00pVBankGP2F/kaObq5EAdGLvI+omE+DAEBmXzKceTPz+weyz8nYZbOkpsBYbvy6gN7h38ZVL6RTqln7XbUzHkQqxRp1B1ZYkBgMW1NtE7r8Jwg26HcS3qPiwYAAYiUZMJyTpfTrVXAAgIFngACAwQBAwDgBQEDAOAHAWJCADZmmS1CxhvLSf1xXlVY4Ug0GNJwhTYd2id0WwreUOkQCKAAAAAAAAAAAAAAAAABBgBQAAAAADcwMzBhYzQ2LWI5NWMtNDRjNy04ZDdiLTYxMjMyNmU2ZTUxMgFiQgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAlAAAAAAAAAAAAAAAAAAQgAUAAAAAAzYjA2OTU1YS03YjRjLTQ1YWEtOTVlNy0wNTI4ZWZhYjAyM2E=",
			filenamePrefix: "decode-message-highload-v2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)

			response, err := h.DecodeMessage(context.Background(), &oas.DecodeMessageReq{Boc: tt.boc})
			if len(tt.wantErr) > 0 {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.Nil(t, err)
			pkgTesting.CompareResults(t, response, tt.filenamePrefix)
		})
	}
}
