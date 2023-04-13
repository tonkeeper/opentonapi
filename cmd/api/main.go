package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/api"
	"github.com/tonkeeper/opentonapi/pkg/app"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
)

func main() {
	cfg := config.Load()
	log := app.Logger(cfg.App.LogLevel)
	book := addressbook.NewAddressBook(log, config.AddressPath, config.JettonPath, config.CollectionPath)
	storage, err := litestorage.NewLiteStorage(
		log,
		litestorage.WithPreloadAccounts(cfg.App.Accounts),
		litestorage.WithTFPools(book.TFPools()),
		litestorage.WithKnownJettons(maps.Keys(book.GetKnownJettons())),
		litestorage.WithLiteServers(cfg.App.LiteServers),
	)
	if err != nil {
		log.Fatal("storage init", zap.Error(err))
	}
	h, err := api.NewHandler(log, api.WithStorage(storage), api.WithAddressBook(book), api.WithExecutor(storage))
	if err != nil {
		log.Fatal("failed to create api handler", zap.Error(err))
	}
	server, err := api.NewServer(log, h, fmt.Sprintf(":%d", cfg.API.Port), api.WithLiteServers(cfg.App.LiteServers))
	if err != nil {
		log.Fatal("failed to create api handler", zap.Error(err))
	}

	metricServer := http.Server{
		Addr:    fmt.Sprintf(":%v", cfg.App.MetricsPort),
		Handler: promhttp.Handler(),
	}
	go func() {
		if err := metricServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("listen and serve", zap.Error(err))
		}
	}()

	log.Info("start server", zap.Int("port", cfg.API.Port))
	server.Run()
}
