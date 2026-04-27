package pyth

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// PriceFeeds provides metadata for Pyth Network price feeds by ID (256 bits hex number)
// see https://docs.pyth.network/price-feeds/core/price-feeds/price-feed-ids
type PriceFeeds interface {
	GetFeed(id string) (PriceFeedAttributes, bool)
}

type PriceFeedAttributes struct {
	DisplaySymbol string `json:"display_symbol"`
	Symbol        string `json:"symbol"`
	AssetType     string `json:"asset_type"`
	Base          string `json:"base"`
	QuoteCurrency string `json:"quote_currency"`
	Description   string `json:"description"`
}

type priceFeedsImpl struct {
	feeds map[string]PriceFeedAttributes // keyed by 64-char lowercase hex
}

func (p *priceFeedsImpl) GetFeed(id string) (PriceFeedAttributes, bool) {
	attrs, ok := p.feeds[id]
	return attrs, ok
}

func NewFromJSON(data []byte) (PriceFeeds, error) {
	var entries []struct {
		ID         string              `json:"id"`
		Attributes PriceFeedAttributes `json:"attributes"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	feeds := make(map[string]PriceFeedAttributes, len(entries))
	for _, e := range entries {
		feeds[e.ID] = e.Attributes
	}
	return &priceFeedsImpl{feeds: feeds}, nil
}

// Default is loaded from the embedded pyth_price_feeds.json
var Default PriceFeeds

//go:embed pyth_price_feeds.json
var defaultFeedsJSON []byte

const hermesURL = "https://hermes.pyth.network/v2/price_feeds"
const hermesFetchTimeout = 5 * time.Second

func init() {
	var err error
	Default, err = NewFromJSON(defaultFeedsJSON)
	if err != nil {
		panic(fmt.Sprintf("pyth: failed to load embedded price feeds: %v", err))
	}
}

// GetUpdatedWithFallback fetches fresh price feed metadata, in case of failure - returns embedded version
func GetUpdatedWithFallback(ctx context.Context, logger *zap.Logger) PriceFeeds {
	ctx, cancel := context.WithTimeout(ctx, hermesFetchTimeout)
	defer cancel()

	feeds, err := fetchFromHermes(ctx)
	if err != nil {
		logger.Warn("failed to fetch pyth price feeds, using embedded fallback", zap.Error(err))
		return Default
	}
	return feeds
}

func fetchFromHermes(ctx context.Context) (PriceFeeds, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hermesURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hermes returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return NewFromJSON(body)
}
