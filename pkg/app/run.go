package app

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)





const (
	shutdownTimeout = time.Second * 5
)

func Logger(level string) *zap.Logger {

	cfg := zap.NewProductionConfig()

	if s := os.Getenv(level); s != "" {
		var lvl zapcore.Level
		if err := lvl.UnmarshalText([]byte(s)); err != nil {
			panic(err)
		}
		cfg.Level.SetLevel(lvl)
	}
	lg, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	return lg
}


