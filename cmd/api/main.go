package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
	"github.com/tonkeeper/opentonapi/pkg/spam"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
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

	var err error
	var client *liteapi.Client
	if len(cfg.App.LiteServers) == 0 {
		log.Warn("USING PUBLIC CONFIG for NewLiteStorage! BE CAREFUL!")
		client, err = liteapi.NewClientWithDefaultMainnet()
	} else {
		client, err = liteapi.NewClient(liteapi.WithLiteServers(cfg.App.LiteServers))
	}
	if err != nil {
		log.Fatal("failed to create liteapi client", zap.Error(err))
	}

	storage, err := litestorage.NewLiteStorage(
		log,
		client,
		litestorage.WithPreloadAccounts(cfg.App.Accounts),
		litestorage.WithTFPools(book.TFPools()),
		litestorage.WithKnownJettons(maps.Keys(book.GetKnownJettons())),
		litestorage.WithBlockChannel(storageBlockCh),
	)
	// The executor is used to resolve DNS records.
	tongo.SetDefaultExecutor(storage)

	if err != nil {
		log.Fatal("storage init", zap.Error(err))
	}
	// mempool receives a copy of any payload that goes through our API method /v2/blockchain/message
	mempool := sources.NewMemPool(log)
	mempoolCh := mempool.Run(context.TODO())

	msgSender, err := blockchain.NewMsgSender(log, cfg.App.LiteServers, map[string]chan<- blockchain.ExtInMsgCopy{
		"mempool": mempoolCh,
	})
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
	)
	if err != nil {
		log.Fatal("failed to create api handler", zap.Error(err))
	}
	source := sources.NewBlockchainSource(log, client)
	pusherBlockCh := source.Run(context.TODO())

	tracer := sources.NewTracer(log, storage, source)
	go tracer.Run(context.TODO())

	idx := indexer.New(log, client)
	go idx.Run(context.TODO(), []chan indexer.IDandBlock{
		pusherBlockCh,
		storageBlockCh,
	})

	server, err := api.NewServer(log, h,
		api.WithTransactionSource(source),
		api.WithBlockHeadersSource(source),
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

	log.Warn("start server", zap.Int("port", cfg.API.Port))
	server.Run(fmt.Sprintf(":%d", cfg.API.Port), cfg.API.UnixSockets)
}
