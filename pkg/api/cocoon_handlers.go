package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-faster/jx"
	"github.com/tonkeeper/gocoon"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

const (
	cocoonPoolRetries     = 5
	cocoonPoolMaxAttempts = 1 + cocoonPoolRetries
)

func cocoonResponseAsJSON(resp []byte) ([]byte, error) {
	if len(resp) == 0 {
		return nil, fmt.Errorf("empty cocoon response")
	}
	trim := bytes.TrimLeft(resp, " \t\r\n")
	body := resp
	if bytes.HasPrefix(trim, []byte("HTTP/")) {
		httpRes, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(resp)), nil)
		if err != nil {
			return nil, fmt.Errorf("parse cocoon http response: %w", err)
		}
		defer httpRes.Body.Close()
		b, err := io.ReadAll(httpRes.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.TrimSpace(b)
	} else {
		i := bytes.IndexFunc(trim, func(c rune) bool { return c == '{' || c == '[' })
		if i < 0 {
			return nil, fmt.Errorf("no JSON value in cocoon response")
		}
		off := len(resp) - len(trim) + i
		body = bytes.TrimSpace(resp[off:])
	}
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	dec := json.NewDecoder(bytes.NewReader(body))
	var out json.RawMessage
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("cocoon response is not valid JSON: %w", err)
	}
	return []byte(out), nil
}

func mergeCocoonModelIntoBody(req jx.Raw, modelQuery oas.OptString) (body []byte, model string) {
	body = []byte(req)
	if len(body) == 0 {
		body = []byte("{}")
	}
	model = ""
	if modelQuery.Set {
		model = modelQuery.Value
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
	return body, model
}

func parseCocoonChatCompletionUsage(body []byte) (tokens int64, cost int64, ok bool) {
	var root struct {
		Usage *struct {
			TotalTokens int64 `json:"total_tokens"`
			TotalCost   int64 `json:"total_cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &root); err != nil || root.Usage == nil {
		return 0, 0, false
	}
	if root.Usage.TotalTokens <= 0 || root.Usage.TotalCost <= 0 {
		return 0, 0, false
	}
	return root.Usage.TotalTokens, root.Usage.TotalCost, true
}

func (h *Handler) recordCocoonChatCompletionUsage(ctx context.Context, jsonBody []byte) {
	if h.tonConsole == nil {
		return
	}
	tokens, cost, ok := parseCocoonChatCompletionUsage(jsonBody)
	if !ok {
		return
	}
	if err := h.tonConsole.CreateCocoonUsageEvent(ctx, tokens, cost); err != nil {
		h.logger.Warn("cocoon usage event", zap.Error(err))
	}
}

func (h *Handler) PostCocoonQuery(ctx context.Context, req jx.Raw, params oas.PostCocoonQueryParams) (oas.PostCocoonQueryRes, error) {
	//if h.cocoonPool == nil {
	//	return &oas.PostCocoonQueryNotImplemented{}, nil
	//}
	//body, model := mergeCocoonModelIntoBody(req, params.Model)
	//conn, err := h.cocoonPool.pick(ctx)
	//if err != nil {
	//	return nil, toError(http.StatusInternalServerError, err)
	//}
	//respBody, err := conn.POST(ctx, model, params.Path.Value, body)
	//if err != nil {
	//	return nil, toError(http.StatusBadGateway, err)
	//}
	//jsonBody, err := cocoonResponseAsJSON(respBody)
	//if err != nil {
	//	return nil, toError(http.StatusBadGateway, err)
	//}
	//ok := oas.PostCocoonQueryOKApplicationJSON(jx.Raw(jsonBody))
	//return &ok, nil
	return &oas.PostCocoonQueryNotImplemented{}, nil
}

func (h *Handler) PostCocoonV1ChatCompletions(ctx context.Context, req jx.Raw) (oas.PostCocoonV1ChatCompletionsRes, error) {
	if h.cocoonPool == nil {
		return &oas.PostCocoonV1ChatCompletionsNotImplemented{}, nil
	}
	body, model := mergeCocoonModelIntoBody(req, oas.OptString{})
	const upstreamPath = "/v1/chat/completions"
	var jsonBody []byte
	var lastErr error
	lastFailedPick := false
	for attempt := 0; attempt < cocoonPoolMaxAttempts; attempt++ {
		conn, err := h.cocoonPool.pick(ctx)
		if err != nil {
			lastErr = err
			lastFailedPick = true
			fmt.Println("cocoon pick error: ", err)
			continue
		}
		lastFailedPick = false
		respBody, err := conn.POST(ctx, model, upstreamPath, body)
		if err != nil {
			lastErr = err
			fmt.Println("cocoon error POST: ", err)
			continue
		}
		jsonBody, err = cocoonResponseAsJSON(respBody)
		if err != nil {
			lastErr = err
			fmt.Println("cocoon json error: ", err)
			continue
		}
		ok := oas.PostCocoonV1ChatCompletionsOKApplicationJSON(jx.Raw(jsonBody))
		h.recordCocoonChatCompletionUsage(ctx, jsonBody)
		return &ok, nil
	}
	if lastFailedPick {
		return nil, toError(http.StatusInternalServerError, lastErr)
	}
	return nil, toError(http.StatusBadGateway, lastErr)
}

func (h *Handler) GetCocoonWorkers(ctx context.Context) (oas.GetCocoonWorkersRes, error) {
	if h.cocoonPool == nil {
		return &oas.GetCocoonWorkersNotImplemented{}, nil
	}
	var types []gocoon.WorkerType
	var lastErr error
	lastFailedPick := false
	for attempt := 0; attempt < cocoonPoolMaxAttempts; attempt++ {
		conn, err := h.cocoonPool.pick(ctx)
		if err != nil {
			lastErr = err
			lastFailedPick = true
			fmt.Println("cocoon pick error: ", err)
			continue
		}
		lastFailedPick = false
		types, err = conn.GetWorkerTypes(ctx)
		if err != nil {
			lastErr = err
			fmt.Println("cocoon GetWorkerTypes error: ", err)
			continue
		}
		return workerTypesToResponse(types), nil
	}
	if lastFailedPick {
		return nil, toError(http.StatusInternalServerError, lastErr)
	}
	return nil, toError(http.StatusBadGateway, lastErr)
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
