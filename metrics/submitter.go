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

type RelayerMetrics struct {
	ResendIntervalSecondsGauge            prometheus.Gauge
	NewSubmittedCheckpointSegmentGaugeVec *prometheus.GaugeVec
	// TODO bug alert
}

func NewRelayerMetrics(registry *prometheus.Registry) *RelayerMetrics {
	registerer := promauto.With(registry)

	metrics := &RelayerMetrics{
		ResendIntervalSecondsGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_submitter_resend_intervals",
			Help: "The intervals the submitter resends a checkpoint in seconds",
		}),
		NewSubmittedCheckpointSegmentGaugeVec: registerer.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vigilante_submitter_new_checkpoint_segment",
				Help: "The metric of a new Babylon checkpoint segment submitted to BTC",
			},
			[]string{
				// the epoch number of the checkpoint segment
				"epoch",
				// the index of the checkpoint segment (either 0 or 1)
				"idx",
				// the id of the checkpoint segment
				"txid",
				// the fee used by submitting the checkpoint segment
				"fee",
			},
		),
	}

	return metrics
}

func NewSubmitterMetrics() *SubmitterMetrics {
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)

	metrics := &SubmitterMetrics{
		Registry: registry,
		SuccessfulCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_submitter_submitted_checkpoints",
			Help: "The total number of raw checkpoints submitted to BTC",
		}),
		FailedCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_submitter_failed_checkpoints",
			Help: "The total number of failed checkpoints to BTC",
		}),
		SecondsSinceLastCheckpointGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_submitter_since_last_checkpoint_seconds",
			Help: "Seconds since the last successfully submitted checkpoint",
		}),
	}
	return metrics
}

func (sm *SubmitterMetrics) RecordMetrics() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			// will be reset when a checkpoint is successfully submitted
			sm.SecondsSinceLastCheckpointGauge.Inc()
		}
	}()
}
