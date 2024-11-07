package rates

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	errorsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rates_getter_errors_total",
	}, []string{"source"})
)
