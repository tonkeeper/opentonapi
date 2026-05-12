package defi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
)

func currencyReserveToNanoTON(ctx context.Context, d Deps, rates map[string]float64, reserve *big.Int, asset core.Currency) float64 {
	reserveF, _ := new(big.Float).SetInt(reserve).Float64()
	switch asset.Type {
	case core.CurrencyTON:
		return reserveF
	case core.CurrencyJetton:
		if asset.Jetton == nil {
			return 0
		}
		tonPrice, ok := rates["TON"]
		if !ok || tonPrice == 0 {
			return 0
		}
		tokenPrice, ok := rates[asset.Jetton.ToRaw()]
		if !ok || tokenPrice == 0 {
			return 0
		}
		meta := d.JettonMeta(ctx, *asset.Jetton)
		return (reserveF / pow10float(meta.Decimals)) * (tokenPrice / tonPrice) * 1e9
	}
	return 0
}

func JettonPositionNanoTON(rates map[string]float64, jettonMasterRaw string, balanceRaw decimal.Decimal, decimals int) int64 {
	tonPrice, ok := rates["TON"]
	if !ok || tonPrice == 0 {
		return 0
	}
	tokenPrice, ok := rates[jettonMasterRaw]
	if !ok || tokenPrice == 0 {
		return 0
	}
	bal, _ := balanceRaw.Float64()
	divisor := pow10float(decimals)
	positionTON := (bal / divisor) * (tokenPrice / tonPrice)
	return int64(positionTON * 1e9)
}

func pow10float(n int) float64 {
	v := 1.0
	for i := 0; i < n; i++ {
		v *= 10
	}
	return v
}

func defiProvider(meta references.DefiProviderMeta) oas.DefiProvider {
	return oas.DefiProvider{
		Name:  meta.Name,
		URL:   meta.URL,
		Image: meta.Image,
	}
}

func fetchJSON(ctx context.Context, url string, dst any) error {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}
