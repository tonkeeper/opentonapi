package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-faster/jx"
	gocoon "github.com/tonkeeper/gocoon/pkg/client"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h *Handler) PostCocoonQuery(ctx context.Context, req jx.Raw, params oas.PostCocoonQueryParams) (oas.PostCocoonQueryRes, error) {
	if h.cocoonPool == nil {
		return &oas.PostCocoonQueryNotImplemented{}, nil
	}
	conn, err := h.cocoonPool.pick(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	body := []byte(req)
	if len(body) == 0 {
		body = []byte("{}")
	}

	model := ""
	if params.Model.Set {
		model = params.Model.Value
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil && obj != nil {
		if model != "" {
			obj["model"] = model
		} else if s, ok := obj["model"].(string); ok {
			model = s
		}
		if b, err := json.Marshal(obj); err == nil {
			body = b
		}
	}

	upstreamPath := params.Path.Value
	respBody, err := conn.POST(ctx, model, upstreamPath, body)
	if err != nil {
		return nil, toError(http.StatusBadGateway, err)
	}

	ok := oas.PostCocoonQueryOKApplicationJSON(jx.Raw(respBody))
	return &ok, nil
}

func (h *Handler) GetCocoonWorkers(ctx context.Context) (oas.GetCocoonWorkersRes, error) {
	if h.cocoonPool == nil {
		return &oas.GetCocoonWorkersNotImplemented{}, nil
	}
	conn, err := h.cocoonPool.pick(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	types, err := conn.GetWorkerTypes(ctx)
	if err != nil {
		return nil, toError(http.StatusBadGateway, err)
	}
	return workerTypesToResponse(types), nil
}

func workerTypesToResponse(types []gocoon.WorkerType) *oas.CocoonWorkersResponse {
	out := make([]oas.CocoonWorkerType, 0, len(types))
	for _, wt := range types {
		ws := make([]oas.CocoonWorkerInstance, 0, len(wt.Workers))
		for _, w := range wt.Workers {
			ws = append(ws, oas.CocoonWorkerInstance{
				Coefficient:       int64(w.Coefficient),
				ActiveRequests:    int64(w.ActiveRequests),
				MaxActiveRequests: int64(w.MaxActiveRequests),
			})
		}
		out = append(out, oas.CocoonWorkerType{Name: wt.Name, Workers: ws})
	}
	return &oas.CocoonWorkersResponse{Types: out}
}
