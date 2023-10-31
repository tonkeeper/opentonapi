package app

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	shutdownTimeout = time.Second * 5
)

func Logger(level string) *zap.Logger {

	cfg := zap.NewProductionConfig()

	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		panic(err)
	}
	cfg.Level.SetLevel(lvl)

	lg, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	return lg
}
