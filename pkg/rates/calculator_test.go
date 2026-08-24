package rates

import (
	"testing"
	"time"
)

// fakeTimestampedSource serves rates through the timestamped endpoint interface.
type fakeTimestampedSource struct {
	rates      map[string]float64
	timestamps map[string]int64
}

func (f *fakeTimestampedSource) GetRates(date int64) (map[string]float64, error) {
	return f.rates, nil
}

func (f *fakeTimestampedSource) GetRatesWithTimestamps(date int64) (map[string]float64, map[string]int64, error) {
	return f.rates, f.timestamps, nil
}

func (f *fakeTimestampedSource) GetRatesChart(token string, currency string, pointsCount int, startDate *int64, endDate *int64) ([]Point, error) {
	return []Point{}, nil
}

func (f *fakeTimestampedSource) GetMarketsTonPrice() ([]Market, error) {
	return []Market{}, nil
}

func TestCalculator_Refresh_UsesTimestampedSourcePrices(t *testing.T) {
	source := &fakeTimestampedSource{
		rates:      map[string]float64{"TON": 1.0},
		timestamps: map[string]int64{"TON": 100},
	}
	c := &calculator{
		source:          source,
		todayRates:      map[string]float64{},
		yesterdayRates:  map[string]float64{},
		weekRates:       map[string]float64{},
		monthRates:      map[string]float64{},
		marketsTonPrice: []Market{},
	}

	c.refresh()

	rates, err := c.GetRates(time.Now().UTC().Unix())
	if err != nil {
		t.Fatalf("GetRates: %v", err)
	}
	if rates["TON"] != 1.0 {
		t.Fatalf("expected today rates from the timestamped source, got %v", rates)
	}
}
