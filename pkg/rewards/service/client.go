package service

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tonkeeper/tongo/config"
)

const (
	configURL = "https://ton.org/global-config.json"
	cacheTTL  = 7 * 24 * time.Hour
)

var globalConfigCache struct {
	sync.Mutex
	conf       *config.GlobalConfigurationFile
	fetchedAt  time.Time
	configPath string
}

// NewClient creates a round-robin client over liteapi that distributes requests
// across all available liteserver connections. Unlike the default liteapi
// pool which routes all traffic through a single "best" connection, this
// ensures all connections are utilized.
func NewClient(liteServers []config.LiteServer) (*RoundRobinClient, error) {
	return NewRoundRobinClient(liteServers)
}

func getCachedConfig(configPath string) (*config.GlobalConfigurationFile, error) {
	globalConfigCache.Lock()
	defer globalConfigCache.Unlock()

	// Invalidate cache if config path changed.
	if globalConfigCache.conf != nil && globalConfigCache.configPath == configPath && time.Since(globalConfigCache.fetchedAt) < cacheTTL {
		log.Printf("using in-memory cached config (age: %s)", time.Since(globalConfigCache.fetchedAt).Round(time.Minute))
		return globalConfigCache.conf, nil
	}

	var conf *config.GlobalConfigurationFile
	var err error

	if configPath != "" {
		if strings.HasPrefix(configPath, "http") {
			conf, err = downloadConfig(&configPath)
		} else {
			log.Printf("loading config from %s...", configPath)
			conf, err = loadConfigFromFile(configPath)
		}
	} else {
		log.Printf("downloading config from %s...", configURL)
		conf, err = downloadConfig(nil)
	}
	if err != nil {
		return nil, err
	}

	globalConfigCache.conf = conf
	globalConfigCache.fetchedAt = time.Now()
	globalConfigCache.configPath = configPath
	log.Printf("config cached in memory (%d liteservers)", len(conf.LiteServers))

	return conf, nil
}

func loadConfigFromFile(path string) (*config.GlobalConfigurationFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	conf, err := config.ParseConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return conf, nil
}

func downloadConfig(configPath *string) (*config.GlobalConfigurationFile, error) {
	var url = configURL
	if configPath != nil {
		url = *configPath
	}
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("download config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read config body: %w", err)
	}
	conf, err := config.ParseConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return conf, nil
}
