package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/api"
	"github.com/tonkeeper/opentonapi/pkg/app"
	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
)

func main() {

	cfg := config.Load()
	log := app.Logger(cfg.App.LogLevel)
	book := addressbook.NewAddressBook(log, config.AddressPath, config.JettonPath, config.CollectionPath)

	storageBlockCh := make(chan indexer.IDandBlock)
	storage, err := litestorage.NewLiteStorage(
		log,
		litestorage.WithPreloadAccounts(cfg.App.Accounts),
		litestorage.WithTFPools(book.TFPools()),
		litestorage.WithKnownJettons(maps.Keys(book.GetKnownJettons())),
		litestorage.WithLiteServers(cfg.App.LiteServers),
		litestorage.WithBlockChannel(storageBlockCh),
	)
	if err != nil {
		log.Fatal("storage init", zap.Error(err))
	}
	mempool := sources.NewMemPool(log)
	// mempoolChannel receives a copy of any payload that goes through our API method /v2/blockchain/message
	mempoolChannel := mempool.Run(context.TODO())

	msgSender, err := blockchain.NewMsgSender(cfg.App.LiteServers, []chan []byte{mempoolChannel})
	if err != nil {
		log.Fatal("failed to create msg sender", zap.Error(err))
	}
	h, err := api.NewHandler(log,
		api.WithStorage(storage),
		api.WithAddressBook(book),
		api.WithExecutor(storage),
		api.WithMessageSender(msgSender),
		api.WithTonConnectSecret(cfg.TonConnect.Secret),
	)
	if err != nil {
		log.Fatal("failed to create api handler", zap.Error(err))
	}
	source, err := sources.NewBlockchainSource(log, cfg.App.LiteServers)
	if err != nil {
		log.Fatal("failed to create blockchain source", zap.Error(err))
	}
	pusherBlockCh := source.Run(context.TODO())

	tracer := sources.NewTracer(log, storage, source)
	go tracer.Run(context.TODO())

	idx, err := indexer.New(log, cfg.App.LiteServers)
	if err != nil {
		log.Fatal("failed to create blockchain indexer", zap.Error(err))
	}
	go idx.Run(context.TODO(), []chan indexer.IDandBlock{
		pusherBlockCh,
		storageBlockCh,
	})

	server, err := api.NewServer(log, h, fmt.Sprintf(":%d", cfg.API.Port),
		api.WithTransactionSource(source),
		api.WithTraceSource(tracer),
		api.WithMemPool(mempool))
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
