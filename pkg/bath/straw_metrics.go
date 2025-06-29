package bath

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var strawSuccess = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "bath_straw_success_total",
		Help: "Number of successful merges per straw index",
	},
	[]string{"index"},
)
