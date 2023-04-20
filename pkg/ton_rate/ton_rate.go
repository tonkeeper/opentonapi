package ton_rate

import (
	"encoding/json"
	"fmt"
	"github.com/google/martian/log"
	"go.uber.org/zap"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type TonRates struct {
	mu    sync.RWMutex
	Rates map[string]string
}

func (r *TonRates) GetRates() map[string]string {
	return r.Rates
}

func InitTonRates(logger *zap.Logger) *TonRates {
	data, err := getRates()
	if err != nil {
		logger.Fatal("failed to init ton rates", zap.Error(err))
		return nil
	}

	rates := &TonRates{Rates: data}

	go func() {
		for {
			time.Sleep(time.Minute * 5)
			rates.refresh()
		}
	}()

	return rates
}

func (r *TonRates) refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := getRates()
	if err != nil {
		return
	}
	r.Rates = data
}

func getRates() (map[string]string, error) {
	btc, err := getBTCrates()
	if err != nil {
		log.Errorf("failed to get btc crates: %v", err)
		return nil, err
	}

	okx, err := getOKXPrice()
	if err != nil {
		log.Errorf("failed to get okx price: %v", err)
		return nil, err
	}

	ratePair := new(big.Float)
	ratePair.SetString(btc.Data.Rates[okx.Pair])
	ratePrice := new(big.Float)
	ratePrice.SetString(okx.Price)

	coinbase := make(map[string]string)
	for k, v := range btc.Data.Rates {
		coinbase[k] = v
	}
	coinbase["TON"] = new(big.Float).Quo(ratePair, ratePrice).Text('f', 5)

	return coinbase, nil
}

type btcRates struct {
	Data struct {
		Currency string            `json:"currency"`
		Rates    map[string]string `json:"rates"`
	} `json:"data"`
}

type huobiPrice struct {
	Ch     string `json:"ch"`
	Status string `json:"status"`
	Ts     int64  `json:"ts"`
	Tick   struct {
		ID   int64 `json:"id"`
		Ts   int64 `json:"ts"`
		Data []struct {
			Ts        int64   `json:"ts"`
			TradeID   int64   `json:"trade-id"`
			Amount    float64 `json:"amount"`
			Price     float64 `json:"ton_rate"`
			Direction string  `json:"direction"`
		} `json:"data"`
	} `json:"tick"`
}

type okxPrice struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		InstType  string `json:"instType"`
		InstId    string `json:"instId"`
		Last      string `json:"last"`
		LastSz    string `json:"lastSz"`
		AskPx     string `json:"askPx"`
		AskSz     string `json:"askSz"`
		BidPx     string `json:"bidPx"`
		BidSz     string `json:"bidSz"`
		Open24h   string `json:"open24h"`
		High24h   string `json:"high24h"`
		Low24h    string `json:"low24h"`
		VolCcy24h string `json:"volCcy24h"`
		Vol24h    string `json:"vol24h"`
		Ts        string `json:"ts"`
		SodUtc0   string `json:"sodUtc0"`
		SodUtc8   string `json:"sodUtc8"`
	} `json:"data"`
}

type pairPrice struct {
	Pair  string `json:"pair"`
	Price string `json:"ton_rate"`
}

func getBTCrates() (*btcRates, error) {
	resp, err := http.Get("https://api.coinbase.com/v2/exchange-rates?currency=BTC")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rates: %v", err)
	}
	defer resp.Body.Close()

	var respBody btcRates
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &respBody, nil
}

func getHuobiPrice() (*pairPrice, error) {
	resp, err := http.Get("https://api.huobi.pro/market/trade?symbol=tonusdt")
	if err != nil {
		return nil, fmt.Errorf("can't load huobi ton_rate")
	}
	defer resp.Body.Close()

	var respBody huobiPrice
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if respBody.Status != "ok" {
		return nil, fmt.Errorf("failed to get huobi ton_rate: %v", err)
	}

	if len(respBody.Tick.Data) == 0 {
		return nil, fmt.Errorf("invalid ton_rate")
	}

	pair := pairPrice{
		Pair:  "USDT",
		Price: fmt.Sprintf("%v", respBody.Tick.Data[0].Price),
	}

	return &pair, nil
}

func getOKXPrice() (*pairPrice, error) {
	resp, err := http.Get("https://www.okx.com/api/v5/market/ticker?instId=TON-USDT")
	if err != nil {
		return nil, fmt.Errorf("can't load okx ton_rate")
	}
	defer resp.Body.Close()

	var respBody okxPrice
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if respBody.Code != "0" {
		return nil, fmt.Errorf("failed to get okx ton_rate: %v", err)
	}

	if len(respBody.Data) == 0 {
		return nil, fmt.Errorf("invalid ton_rate")
	}

	price, err := strconv.ParseFloat(respBody.Data[0].Last, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ton_rate")
	}

	pair := pairPrice{
		Pair:  "USDT",
		Price: fmt.Sprintf("%v", price),
	}

	return &pair, nil
}
