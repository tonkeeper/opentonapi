package config

import (
	"github.com/caarlos0/env/v6"
	"log"
)

var API struct {
	Port int `env:"PORT" envDefault:"8081"`
}

var App struct {
	LogLevel string `env:"LOG_LEVEL" envDefault:"INFO"`
}

func Load() {
	if err := env.Parse(&API); err != nil {
		log.Panicf("[‼️  Config parsing failed] %+v\n", err)
	}

}
