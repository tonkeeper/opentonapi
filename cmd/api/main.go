package main

import (
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"opentonapi/internal/config"
	"opentonapi/pkg/api"
	"opentonapi/pkg/app"
	"opentonapi/pkg/oas"
)

func main() {
	config.Load()
	log := app.Logger(config.App.LogLevel)

	h := api.NewHandler("temp")

	oasServer, err := oas.NewServer(h)
	if err != nil {
		log.Fatal("server init", zap.Error(err))
	}
	httpServer := http.Server{
		Addr:    fmt.Sprintf(":%v", config.API.Port),
		Handler: oasServer,
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("listen and serve", zap.Error(err))
	}

}
