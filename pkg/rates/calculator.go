package rates

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type ratesSource interface {
	GetRates(date int64) (map[string]float64, error)
	GetRatesChart(token string, currency string, pointsCount int, startDate *int64, endDate *int64) ([][]any, error)
	GetMarketsTonPrice() ([]Market, error)
}

type calculator struct {
	mu sync.RWMutex
	// The source contains complex logic hidden in the ratesSource interface, which is not available in the open source version
	// See the Mock description for details
	source                                            ratesSource
	todayRates, yesterdayRates, weekRates, monthRates map[string]float64
	marketsTonPrice                                   []Market
}

func InitCalculator(source ratesSource) *calculator {
	if source == nil {
		log.Fatalf("source is not configured")
	}

	c := &calculator{
		source:          source,
		todayRates:      map[string]float64{},
		yesterdayRates:  map[string]float64{},
		weekRates:       map[string]float64{},
		monthRates:      map[string]float64{},
		marketsTonPrice: []Market{},
	}

	go func() {
		for {
			c.refresh()
			time.Sleep(time.Minute * 5)
		}
	}()

	return c
}

func (c *calculator) refresh() {
	today := time.Now().UTC()
	yesterday := today.AddDate(0, 0, -1).Unix()
	weekAgo := today.AddDate(0, 0, -7).Unix()
	monthAgo := today.AddDate(0, 0, -30).Unix()

	marketsTonPrice, err := c.source.GetMarketsTonPrice()
	if err != nil {
		return
	}
	todayRates, err := c.source.GetRates(today.Unix())
	if err != nil {
		return
	}
	yesterdayRates, err := c.source.GetRates(yesterday)
	if err != nil {
		return
	}
	weekRates, err := c.source.GetRates(weekAgo)
	if err != nil {
		return
	}
	monthRates, err := c.source.GetRates(monthAgo)
	if err != nil {
		return
	}

	c.mu.Lock()
	c.todayRates = todayRates
	c.yesterdayRates = yesterdayRates
	c.weekRates = weekRates
	c.monthRates = monthRates
	c.marketsTonPrice = marketsTonPrice
	c.mu.Unlock()
}

func (c *calculator) GetRates(date int64) (map[string]float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	truncatedDate := time.Unix(date, 0).Truncate(time.Hour * 24).UTC()

	today := time.Now().Truncate(time.Hour * 24).UTC()
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, 0, -30)

	switch {
	case truncatedDate.Equal(today):
		return c.todayRates, nil
	case truncatedDate.Equal(yesterday):
		return c.yesterdayRates, nil
	case truncatedDate.Equal(weekAgo):
		return c.weekRates, nil
	case truncatedDate.Equal(monthAgo):
		return c.monthRates, nil
	}

	return nil, fmt.Errorf("invalid period")
}

func (c *calculator) GetRatesChart(token string, currency string, pointsCount int, startDate *int64, endDate *int64) ([][]any, error) {
	return c.source.GetRatesChart(token, currency, pointsCount, startDate, endDate)
}

func (c *calculator) GetMarketsTonPrice() ([]Market, error) {
	return c.source.GetMarketsTonPrice()
}

type Mock struct {
	// TonApiToken the token for TonApi to increase HTTP limits is obtained from https://tonconsole.com/tonapi
	TonApiToken string
	// URL to the CSV file from the analytics service https://tonconsole.com/analytics (data is sourced from the TonApi analytics database)
	StonV1FiResultUrl, StonV2FiResultUrl string
	// URL to the CSV file from the analytics service https://tonconsole.com/analytics (data is sourced from the TonApi analytics database)
	DedustResultUrl string
}

// GetRates cannot request data for a specific date in the open source version
// It will always return data for the current day
func (m Mock) GetRates(date int64) (map[string]float64, error) {
	return m.GetCurrentRates()
}

func (m Mock) GetMarketsTonPrice() ([]Market, error) {
	return m.GetCurrentMarketsTonPrice()
}

// GetRatesChart cannot be used to request charts for jettons in the open source version
// To use this method, you must save today's data from GetRates and then override the method
func (m Mock) GetRatesChart(token string, currency string, pointsCount int, startDate *int64, endDate *int64) ([][]any, error) {
	return [][]any{}, nil
}
