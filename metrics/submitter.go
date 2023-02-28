package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type SubmitterMetrics struct {
	Registry                        *prometheus.Registry
	SuccessfulCheckpointsCounter    prometheus.Counter
	FailedCheckpointsCounter        prometheus.Counter
	SecondsSinceLastCheckpointGauge prometheus.Gauge
}

func NewSubmitterMetrics() *SubmitterMetrics {
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)

	metrics := &SubmitterMetrics{
		Registry: registry,
		SuccessfulCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_submitter_submitted_checkpoints",
			Help: "The total number of submitted checkpoints to BTC",
		}),
		FailedCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_submitter_failed_checkpoints",
			Help: "The total number of failed checkpoints to BTC",
		}),
		SecondsSinceLastCheckpointGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_submitter_since_last_checkpoint_seconds",
			Help: "Seconds since the last successful submitted checkpoint",
		}),
	}
	return metrics
}

func (sm *SubmitterMetrics) RecordMetrics() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			sm.SecondsSinceLastCheckpointGauge.Inc()
		}
	}()
}
