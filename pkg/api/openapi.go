package api

import (
	"bytes"
	"context"
	"github.com/go-faster/jx"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"net/http"
	"os"
)

func (h *Handler) GetOpenapiJson(ctx context.Context) (jx.Raw, error) {
	file, err := os.ReadFile("openapi/openapi.json")
	if err != nil {
		return jx.Raw{}, toError(http.StatusInternalServerError, err)
	}
	d := jx.DecodeBytes(file)
	result, err := d.Raw()
	if err != nil {
		return jx.Raw{}, toError(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (h *Handler) GetOpenapiYml(ctx context.Context) (oas.GetOpenapiYmlOK, error) {
	file, err := os.ReadFile("openapi/openapi.yml")
	if err != nil {
		return oas.GetOpenapiYmlOK{}, toError(http.StatusInternalServerError, err)
	}
	return oas.GetOpenapiYmlOK{
		Data: bytes.NewReader(file),
	}, nil
}
