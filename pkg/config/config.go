package config

import (
	"fmt"
	"github.com/tonkeeper/tongo/config"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v6"
	"github.com/tonkeeper/tongo"
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
}

var Loader struct {
	IpfsGate       string `env:"IPFS_GATE" envDefault:"https://ipfs.io/ipfs/"`
	TonStorageGate string `env:"TONSTORAGE_GATE" envDefault:"http://storage.ton/gateway/"`
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
			serverStrings := strings.Split(v, ",")
			if len(serverStrings) == 0 {
				return nil, fmt.Errorf("empty liteservers list")
			}
			var servers []config.LiteServer
			for _, s := range serverStrings {
				params := strings.Split(s, ":")
				if len(params) != 3 {
					return nil, fmt.Errorf("invalid liteserver config string")
				}
				ip := net.ParseIP(params[0])
				if ip == nil {
					return nil, fmt.Errorf("invalid lite server ip")
				}
				if ip.To4() == nil {
					return nil, fmt.Errorf("IPv6 not supported")
				}
				_, err := strconv.ParseInt(params[1], 10, 32)
				if err != nil {
					return nil, fmt.Errorf("invalid lite server port: %v", err)
				}
				servers = append(servers, config.LiteServer{
					Host: fmt.Sprintf("%v:%v", params[0], params[1]),
					Key:  params[2],
				})
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

	if err := env.Parse(&Loader); err != nil {
		log.Panicf("[‼️  Config parsing failed] %+v\n", err)
	}

	return c
}
