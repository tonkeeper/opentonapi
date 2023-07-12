package rates

import (
	"sync"
	"time"
)

type ratesSource interface {
	GetRates(date time.Time) (map[string]float64, error)
}

type calculator struct {
	mu                     sync.RWMutex
	source                 ratesSource
	oldRates, currentRates map[string]float64
}

func InitCalculator(storage storage) *calculator {
	c := &calculator{
		source:       ratesMock{storage},
		oldRates:     map[string]float64{},
		currentRates: map[string]float64{},
	}

	go func() {
		for {
			c.refresh()
			time.Sleep(time.Minute * 5)
		}
	}()

	return c
}

func (c *calculator) GetRates(date time.Time) (map[string]float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	truncatedDate := date.Truncate(24 * time.Hour)

	if truncatedDate.Equal(today) {
		return c.currentRates, nil
	} else {
		return c.oldRates, nil
	}
}

type ratesMock struct {
	storage storage
}

func (r ratesMock) GetRates(date time.Time) (map[string]float64, error) {
	return r.GetCurrentRates()
}

func (c *calculator) refresh() {
	for {
		now := time.Now().UTC()
		yesterday := now.AddDate(0, 0, -1)

		currentRates, err := c.source.GetRates(now)
		if err != nil {
			continue
		}
		oldRates, err := c.source.GetRates(yesterday)
		if err != nil {
			continue
		}

		c.mu.RLock()
		c.currentRates = currentRates
		c.oldRates = oldRates
		c.mu.RUnlock()

		time.Sleep(5 * time.Minute)
	}
}
