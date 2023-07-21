package sentry

import (
	"fmt"
	"os"
	"time"
)

type SentryInfoData map[string]interface{}

var inited = false

func init() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		fmt.Printf("failed to sentry init: %s", err)
	}
	inited = true
	sentry.Flush(2 * time.Second)
}

func Send(title string, data SentryInfoData, logLevel sentry.Level) {
	if !inited {
		return
	}

	go func(localHub *sentry.Hub) {
		localHub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetLevel(logLevel)
			scope.SetExtras(data)
		})
		localHub.CaptureMessage(title)
	}(sentry.CurrentHub().Clone())
}
