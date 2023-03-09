package main

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/tonkeeper/opentonapi/pkg/chainstate"
	"github.com/tonkeeper/opentonapi/pkg/config"

	"net/http"

	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/api"
	"github.com/tonkeeper/opentonapi/pkg/app"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func main() {
	cfg := config.Load()
	log := app.Logger(cfg.App.LogLevel)

	storage, err := litestorage.NewLiteStorage(cfg.App.Accounts, log)
	if err != nil {
		log.Fatal("storage init", zap.Error(err))
	}

	book := addressbook.NewAddressBook(log, config.AddressPath, config.JettonPath, config.CollectionPath)
	h := api.NewHandler(storage, chainstate.NewChainState(), book)

	oasServer, err := oas.NewServer(h, oas.WithMiddleware(api.Logging(log), api.Metrics), oas.WithErrorHandler(api.ErrorsHandler))
	if err != nil {
		log.Fatal("server init", zap.Error(err))
	}
	httpServer := http.Server{
		Addr:    fmt.Sprintf(":%v", cfg.API.Port),
		Handler: oasServer,
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
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("listen and serve", zap.Error(err))
	}

}
