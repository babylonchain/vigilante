package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MonitorMetrics struct {
	Registry                 *prometheus.Registry
	ValidEpochsCounter       prometheus.Counter
	InvalidEpochsCounter     prometheus.Counter
	ValidBTCHeadersCounter   prometheus.Counter
	InvalidBTCHeadersCounter prometheus.Counter
	LivenessAttacksCounter   prometheus.Counter
}

func NewMonitorMetrics() *MonitorMetrics {
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)

	metrics := &MonitorMetrics{
		Registry: registry,
		ValidEpochsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_valid_epochs",
			Help: "The total number of valid epochs",
		}),
		InvalidEpochsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_invalid_epochs",
			Help: "The total number of invalid epochs",
		}),
		ValidBTCHeadersCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_valid_btc_headers",
			Help: "The total number of valid BTC headers",
		}),
		InvalidBTCHeadersCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_invalid_btc_headers",
			Help: "The total number of invalid BTC headers",
		}),
		LivenessAttacksCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_liveness_attacks",
			Help: "The total number of detected liveness attacks",
		}),
	}
	return metrics
}
