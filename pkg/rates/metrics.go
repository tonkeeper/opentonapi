package rates

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	errorsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonapi_rates_errors_total",
			Help: "Total number of errors encountered while fetching token rates from various sources",
		}, []string{"source"},
	)
)
