package rates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/martian/log"
	"github.com/shopspring/decimal"
)

type TonRates struct { // the values are equated to the TON
	mu    sync.RWMutex
	Rates map[string]float64 // {"USD": 2.25}
	Pools map[string]float64 // {"EQDo_ZJyQ_YqBzBwbVpMmhbhIddKtRP99HugZJ14aFscxi7B": 1.0}
}

func (r *TonRates) GetRates() map[string]float64 {
	return r.Rates
}

func (r *TonRates) GetPools() map[string]float64 {
	return r.Pools
}

func InitTonRates() *TonRates {
	rates := &TonRates{Rates: map[string]float64{}, Pools: map[string]float64{}}

	go func() {
		for {
			rates.refresh()
			time.Sleep(time.Minute * 5)
		}
	}()

	return rates
}

func (r *TonRates) refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	rates, err := getRates()
	if err != nil {
		return
	}
	pools, err := getPools()
	if err != nil {
		return
	}
	r.Rates = rates
	r.Pools = pools
}

func getRates() (map[string]float64, error) {
	okx, err := getOKXPrice()
	if err != nil {
		log.Errorf("failed to get okx price: %v", err)
		return nil, err
	}

	huobi, err := getHuobiPrice()
	if err != nil {
		log.Errorf("failed to get huobi price: %v", err)
		return nil, err
	}

	meanTonPriceToUSD := (huobi + okx) / 2

	fiatPricesToUSD, err := getFiatPricesToUSD()
	if err != nil {
		return nil, err
	}

	rates := make(map[string]float64)
	for _, price := range fiatPricesToUSD {
		rates[price.Currency] = meanTonPriceToUSD * price.Price
	}
	rates["TON"] = meanTonPriceToUSD

	return rates, nil
}

func getPools() (map[string]float64, error) {
	resp, err := http.Get("https://api.dedust.io/v2/pools")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rates: %v", err)
	}
	defer resp.Body.Close()

	var respBody []Pool
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody {
		if len(pool.Assets) != 2 {
			log.Errorf("count of assets not 2")
			continue
		}
		firstAsset := pool.Assets[0]
		secondAsset := pool.Assets[1]

		if firstAsset.Metadata == nil || firstAsset.Metadata.Symbol != "TON" {
			continue
		}

		firstReserve := pool.Reserves[0]
		secondReserve := pool.Reserves[1]

		if firstReserve == "0" || secondReserve == "0" {
			continue
		}

		decimals := int32(9)
		if secondAsset.Metadata != nil && secondAsset.Metadata.Decimals != 0 {
			decimals = secondAsset.Metadata.Decimals
		}

		firstReserveConverted, _ := strconv.ParseFloat(firstReserve, 64)
		secondReserveConverted, _ := strconv.ParseFloat(secondReserve, 64)

		decimalFirstReserve := decimal.NewFromFloat(firstReserveConverted)
		decimalSecondReserve := decimal.NewFromFloat(secondReserveConverted)

		price, _ := decimalSecondReserve.Div(decimalFirstReserve).Round(decimals).Float64()

		mapOfPool[secondAsset.Address] = price
	}

	return mapOfPool, nil
}

type Pool struct {
	Assets []struct {
		Type     string `json:"type"`
		Address  string `json:"address"`
		Metadata *struct {
			Name     string `json:"name"`
			Symbol   string `json:"symbol"`
			Decimals int32  `json:"decimals"`
		} `json:"metadata"`
	} `json:"assets"`
	Reserves []string `json:"reserves"`
}

type huobiPrice struct {
	Status string `json:"status"`
	Tick   struct {
		Data []struct {
			Ts     int64   `json:"ts"`
			Amount float64 `json:"amount"`
			Price  float64 `json:"price"`
		} `json:"data"`
	} `json:"tick"`
}

type okxPrice struct {
	Code string `json:"code"`
	Data []struct {
		Last string `json:"last"`
	} `json:"data"`
}

type fiatPriceToUSD struct {
	Currency string  `json:"currency"`
	Price    float64 `json:"price"`
}

func getHuobiPrice() (float64, error) {
	resp, err := http.Get("https://api.huobi.pro/market/trade?symbol=tonusdt")
	if err != nil {
		return 0, fmt.Errorf("can't load huobi price")
	}
	defer resp.Body.Close()

	var respBody huobiPrice
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return 0, fmt.Errorf("failed to decode response: %v", err)
	}

	if respBody.Status != "ok" {
		return 0, fmt.Errorf("failed to get huobi price: %v", err)
	}

	if len(respBody.Tick.Data) == 0 {
		return 0, fmt.Errorf("invalid price")
	}

	return respBody.Tick.Data[0].Price, nil
}

func getOKXPrice() (float64, error) {
	resp, err := http.Get("https://www.okx.com/api/v5/market/ticker?instId=TON-USDT")
	if err != nil {
		return 0, fmt.Errorf("can't load okx price")
	}
	defer resp.Body.Close()

	var respBody okxPrice
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return 0, fmt.Errorf("failed to decode response: %v", err)
	}

	if respBody.Code != "0" {
		return 0, fmt.Errorf("failed to get okx price: %v", err)
	}

	if len(respBody.Data) == 0 {
		return 0, fmt.Errorf("invalid price")
	}

	price, err := strconv.ParseFloat(respBody.Data[0].Last, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price")
	}

	return price, nil
}

func getFiatPricesToUSD() ([]fiatPriceToUSD, error) {
	resp, err := http.Get("https://api.coinbase.com/v2/exchange-rates?currency=USD")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respBody struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	var prices []fiatPriceToUSD
	for currency, rate := range respBody.Data.Rates {
		rateConverted, err := strconv.ParseFloat(rate, 64)
		if err != nil {
			fmt.Printf("failed to convert str to float64 %v, err: %v", rate, err)
			continue
		}
		price := fiatPriceToUSD{
			Currency: currency,
			Price:    rateConverted,
		}
		prices = append(prices, price)
	}

	return prices, nil
}
