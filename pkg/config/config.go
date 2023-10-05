package config

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v6"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/config"
)

type Config struct {
	API struct {
		Port int `env:"PORT" envDefault:"8081"`
	}
	App struct {
		LogLevel    string              `env:"LOG_LEVEL" envDefault:"INFO"`
		MetricsPort int                 `env:"METRICS_PORT" envDefault:"9010"`
		Accounts    accountsList        `env:"ACCOUNTS"`
		LiteServers []config.LiteServer `env:"LITE_SERVERS"`
	}
	AWS struct {
		AccessKeyID     string `env:"AWS_ACCESS_KEY_ID"`
		SecretAccessKey string `env:"AWS_SECRET_ACCESS_KEY"`
	}
	TonConnect struct {
		Secret string `env:"TON_CONNECT_SECRET"`
	}
}

type accountsList []tongo.AccountID

const (
	AddressPath    = "https://raw.githubusercontent.com/tonkeeper/ton-assets/main/accounts.json"
	CollectionPath = "https://raw.githubusercontent.com/tonkeeper/ton-assets/main/collections.json"
	JettonPath     = "https://raw.githubusercontent.com/tonkeeper/ton-assets/main/jettons.json"
)

func Load() Config {
	var c Config
	if err := env.ParseWithFuncs(&c, map[reflect.Type]env.ParserFunc{
		reflect.TypeOf([]config.LiteServer{}): func(v string) (interface{}, error) {
			servers, err := config.ParseLiteServersEnvVar(v)
			if err != nil {
				return nil, err
			}
			if len(servers) == 0 {
				return nil, fmt.Errorf("empty liteservers list")
			}
			return servers, nil
		},
		reflect.TypeOf(accountsList{}): func(v string) (interface{}, error) {
			var accs accountsList
			for _, s := range strings.Split(v, ",") {
				a, err := tongo.ParseAccountID(s)
				if err != nil {
					return nil, err
				}
				accs = append(accs, a)
			}
			return accs, nil
		}}); err != nil {
		log.Panicf("[‼️  Config parsing failed] %+v\n", err)
	}

	return c
}
