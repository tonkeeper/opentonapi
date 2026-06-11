package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/api"
	"github.com/tonkeeper/opentonapi/pkg/app"
	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/pyth"
	"github.com/tonkeeper/opentonapi/pkg/spam"
	"github.com/tonkeeper/tongo"
	ton "github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"
)

func main() {

	cfg := config.Load()
	log := app.Logger(cfg.App.LogLevel)

	storageBlockCh := make(chan indexer.IDandBlock)

	var err error
	var client *liteapi.Client
	if len(cfg.App.LiteServers) == 0 {
		log.Warn("USING PUBLIC CONFIG for NewLiteStorage! BE CAREFUL!")
		client, err = liteapi.NewClient(
			liteapi.Mainnet(),
			liteapi.WithObserver(litestorage.LiteclientObserver{}),
		)
	} else {
		client, err = liteapi.NewClient(
			liteapi.WithLiteServers(cfg.App.LiteServers),
			liteapi.WithObserver(litestorage.LiteclientObserver{}),
		)
	}
	if err != nil {
		log.Fatal("failed to create liteapi client", zap.Error(err))
	}
	var archiveLiteServers []ton.LiteServer
	if len(cfg.App.ArchiveLiteServers) != 0 {
		archiveLiteServers = cfg.App.ArchiveLiteServers
	} else if len(cfg.App.LiteServers) != 0 {
		archiveLiteServers = cfg.App.LiteServers
	} else {
		var opt liteapi.Options
		liteapi.Mainnet()(&opt)
		archiveLiteServers = opt.LiteServers
	}

	pythFeeds := pyth.GetUpdatedWithFallback(context.Background(), log)
	storage, err := litestorage.NewLiteStorage(
		log,
		client,
		litestorage.WithPreloadAccounts(cfg.App.Accounts),
		litestorage.WithBlockChannel(storageBlockCh),
		litestorage.WithPythPriceFeeds(pythFeeds),
	)
	book := addressbook.NewAddressBook(log, config.AddressPath, config.JettonPath, config.CollectionPath, storage)
	// The executor is used to resolve DNS records.
	tongo.SetDefaultExecutor(storage)

	if err != nil {
		log.Fatal("storage init", zap.Error(err))
	}
	msgSender, err := blockchain.NewMsgSender(log, cfg.App.LiteServers, map[string]chan<- blockchain.ExtInMsgCopy{})
	if err != nil {
		log.Fatal("failed to create msg sender", zap.Error(err))
	}
	spamFilter := spam.NewSpamFilter()
	h, err := api.NewHandler(log,
		api.WithStorage(storage),
		api.WithAddressBook(book),
		api.WithExecutor(storage),
		api.WithMessageSender(msgSender),
		api.WithSpamFilter(spamFilter),
		api.WithTonConnectSecret(cfg.TonConnect.Secret),
		api.WithArchiveLiteServers(archiveLiteServers),
		api.WithPublicAPIURL(cfg.PublicAPIURL),
	)
	if err != nil {
		log.Fatal("failed to create api handler", zap.Error(err))
	}
	idx := indexer.New(log, client)
	go idx.Run(context.TODO(), []chan indexer.IDandBlock{
		storageBlockCh,
	})

	server, err := api.NewServer(log, h)
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

	log.Warn("start server", zap.Int("port", cfg.API.Port))
	server.Run(fmt.Sprintf(":%d", cfg.API.Port), cfg.API.UnixSockets)
	select {}
}
