package rates

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/gommon/log"
)

const (
	bitfinex string = "Bitfinex"
	gateio   string = "Gate.io"
	bybit    string = "Bybit"
	kucoin   string = "KuCoin"
	okx      string = "OKX"
	huobi    string = "Huobi"
)

type Market struct {
	ID         int64
	Name       string
	UsdPrice   float64
	ApiURL     string
	DateUpdate time.Time
}

func (m *Mock) GetCurrentMarketsTonPrice() []Market {
	now := time.Now()
	markets := []Market{
		{
			ID:         1,
			Name:       bitfinex,
			ApiURL:     "https://api-pub.bitfinex.com/v2/ticker/tTONUSD",
			DateUpdate: now,
		},
		{
			ID:         2,
			Name:       gateio,
			ApiURL:     "https://api.gateio.ws/api/v4/spot/tickers?currency_pair=TON_USDT",
			DateUpdate: now,
		},
		{
			ID:         3,
			Name:       bybit,
			ApiURL:     "https://api.bybit.com/derivatives/v3/public/tickers?symbol=TONUSDT",
			DateUpdate: now,
		},
		{
			ID:         4,
			Name:       kucoin,
			ApiURL:     "https://www.kucoin.com/_api/trade-front/market/getSymbolTick?symbols=TON-USDT",
			DateUpdate: now,
		},
		{
			ID:         5,
			Name:       okx,
			ApiURL:     "https://www.okx.com/api/v5/market/ticker?instId=TON-USDT",
			DateUpdate: now,
		},
		{
			ID:         6,
			Name:       huobi,
			ApiURL:     "https://api.huobi.pro/market/trade?symbol=tonusdt",
			DateUpdate: now,
		},
	}
	for idx, market := range markets {
		respBody, err := sendRequest(market.ApiURL)
		if err != nil {
			continue
		}
		switch market.Name {
		case bitfinex:
			market.UsdPrice, err = convertedTonBitFinexResponse(respBody)
			if err != nil || market.UsdPrice == 0 {
				continue
			}
		case gateio:
			market.UsdPrice, err = convertedTonGateIOResponse(respBody)
			if err != nil || market.UsdPrice == 0 {
				continue
			}

		case bybit:
			market.UsdPrice, err = convertedTonBybitResponse(respBody)
			if err != nil || market.UsdPrice == 0 {
				continue
			}

		case kucoin:
			market.UsdPrice, err = convertedTonKuCoinResponse(respBody)
			if err != nil || market.UsdPrice == 0 {
				continue
			}

		case okx:
			market.UsdPrice, err = convertedTonOKXResponse(respBody)
			if err != nil || market.UsdPrice == 0 {
				continue
			}

		case huobi:
			market.UsdPrice, err = convertedTonHuobiResponse(respBody)
			if err != nil || market.UsdPrice == 0 {
				continue
			}
		}
		markets[idx] = market
	}
	sort.Slice(markets, func(i, j int) bool {
		return markets[i].ID > markets[j].ID
	})
	return markets
}

func sendRequest(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("[sendRequest] failed to get market price: %v", url)
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("[sendRequest] failed to decode response body: %v", url)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Errorf("[sendRequest] failed to get market price: %v, %v, response body: %v", resp.StatusCode, url, string(respBody))
		return nil, fmt.Errorf("bad status code: %v", resp.StatusCode)
	}
	return respBody, nil
}

func convertedTonGateIOResponse(respBody []byte) (float64, error) {
	var data []struct {
		Last string `json:"last"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return 0, err
	}
	if len(data) == 0 {
		log.Errorf("[convertedTonGateIOResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data[0].Last, 64)
	if err != nil {
		log.Errorf("[convertedTonGateIOResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}
	return price, nil
}

func convertedTonBybitResponse(respBody []byte) (float64, error) {
	var data struct {
		RetMsg string `json:"retMsg"`
		Result struct {
			List []struct {
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return 0, err
	}
	if data.RetMsg != "OK" {
		log.Errorf("[convertedTonBybitResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Result.List) == 0 {
		log.Errorf("[convertedTonBybitResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data.Result.List[0].LastPrice, 64)
	if err != nil {
		log.Errorf("[convertedTonBybitResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}
	return price, nil
}

func convertedTonBitFinexResponse(respBody []byte) (float64, error) {
	var prices []float64
	if err := json.Unmarshal(respBody, &prices); err != nil {
		return 0, err
	}
	if len(prices) == 0 {
		log.Errorf("[convertedTonBitFinexResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	if len(prices) >= 9 { // last market price
		return prices[9], nil
	}
	return prices[0], nil
}

func convertedTonKuCoinResponse(respBody []byte) (float64, error) {
	var data struct {
		Success bool `json:"success"`
		Data    []struct {
			LastTradedPrice string `json:"lastTradedPrice"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return 0, err
	}
	if !data.Success {
		log.Errorf("[convertedTonKuCoinResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Data) == 0 {
		log.Errorf("[convertedTonKuCoinResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data.Data[0].LastTradedPrice, 64)
	if err != nil {
		log.Errorf("[convertedTonKuCoinResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}
	return price, nil
}

func convertedTonOKXResponse(respBody []byte) (float64, error) {
	var data struct {
		Code string `json:"code"`
		Data []struct {
			Last string `json:"last"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return 0, err
	}
	if data.Code != "0" {
		log.Errorf("[convertedTonOKXResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Data) == 0 {
		log.Errorf("[convertedTonOKXResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data.Data[0].Last, 64)
	if err != nil {
		log.Errorf("[convertedTonOKXResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}

	return price, nil
}

func convertedTonHuobiResponse(respBody []byte) (float64, error) {
	var data struct {
		Status string `json:"status"`
		Tick   struct {
			Data []struct {
				Ts     int64   `json:"ts"`
				Amount float64 `json:"amount"`
				Price  float64 `json:"price"`
			} `json:"data"`
		} `json:"tick"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return 0, err
	}
	if data.Status != "ok" {
		log.Errorf("[convertedTonHuobiResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Tick.Data) == 0 {
		log.Errorf("[convertedTonHuobiResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}

	return data.Tick.Data[0].Price, nil
}
