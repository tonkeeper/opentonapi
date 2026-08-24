package rates

import (
	"testing"
)

// fakeTimestampedSource returns a configurable rates map with timestamps and counts fetches.
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

func TestCalculator_MinuteAgoRates_ShiftAndFallback(t *testing.T) {
	source := &fakeTimestampedSource{
		rates:      map[string]float64{"TON": 1.0},
		timestamps: map[string]int64{"TON": 100},
	}
	c := &calculator{
		source:          source,
		todayRates:      map[string]float64{},
		todayTimestamps: map[string]int64{},
		yesterdayRates:  map[string]float64{},
		weekRates:       map[string]float64{},
		monthRates:      map[string]float64{},
		marketsTonPrice: []Market{},
	}

	// Before any refresh both snapshots are empty but usable.
	rates, timestamps := c.GetMinuteAgoRatesWithTimestamps()
	if rates == nil || timestamps == nil {
		t.Fatal("minute-ago accessor must never return nil maps")
	}

	// Cold start: after the first refresh there is no previous snapshot yet,
	// so the minute-ago accessor falls back to the current one.
	c.refresh()
	rates, timestamps = c.GetMinuteAgoRatesWithTimestamps()
	if rates["TON"] != 1.0 || timestamps["TON"] != 100 {
		t.Fatalf("expected fallback to current snapshot, got rates=%v timestamps=%v", rates, timestamps)
	}

	// Second refresh with new prices: the previous snapshot must shift into minute-ago.
	source.rates = map[string]float64{"TON": 2.0}
	source.timestamps = map[string]int64{"TON": 160}
	c.refresh()

	rates, timestamps = c.GetMinuteAgoRatesWithTimestamps()
	if rates["TON"] != 1.0 || timestamps["TON"] != 100 {
		t.Fatalf("expected previous snapshot in minute-ago, got rates=%v timestamps=%v", rates, timestamps)
	}
	rates, timestamps = c.GetTodayRatesWithTimestamps()
	if rates["TON"] != 2.0 || timestamps["TON"] != 160 {
		t.Fatalf("expected fresh snapshot in today, got rates=%v timestamps=%v", rates, timestamps)
	}
}
