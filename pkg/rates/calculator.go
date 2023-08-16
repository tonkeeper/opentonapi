package rates

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type ratesSource interface {
	GetRates(date time.Time) (map[string]float64, error)
	GetRatesChart(token string, currency string, startDate *int64, endDate *int64) ([][]any, error)
}

type calculator struct {
	mu                                                sync.RWMutex
	source                                            ratesSource
	todayRates, yesterdayRates, weekRates, monthRates map[string]float64
}

func InitCalculator(source ratesSource) *calculator {
	if source == nil {
		log.Fatalf("source is not configured")
	}

	c := &calculator{
		source:         source,
		todayRates:     map[string]float64{},
		yesterdayRates: map[string]float64{},
		weekRates:      map[string]float64{},
		monthRates:     map[string]float64{},
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
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, 0, -30)

	todayRates, err := c.source.GetRates(today)
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

	c.mu.RLock()
	c.todayRates = todayRates
	c.yesterdayRates = yesterdayRates
	c.weekRates = weekRates
	c.monthRates = monthRates
	c.mu.RUnlock()
}

func (c *calculator) GetRates(date time.Time) (map[string]float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	truncatedDate := date.Truncate(24 * time.Hour)

	today := time.Now().UTC().Truncate(24 * time.Hour)
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

func (c *calculator) GetRatesChart(token string, currency string, startDate *int64, endDate *int64) ([][]any, error) {
	return c.source.GetRatesChart(token, currency, startDate, endDate)
}

type Mock struct {
	Storage storage
}

func (m Mock) GetRates(date time.Time) (map[string]float64, error) {
	return m.GetCurrentRates()
}

func (m Mock) GetRatesChart(token string, currency string, startDate *int64, endDate *int64) ([][]any, error) {
	return [][]any{}, nil
}
