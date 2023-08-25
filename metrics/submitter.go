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
	*RelayerMetrics
}

type RelayerMetrics struct {
	ResendIntervalSecondsGauge            prometheus.Gauge
	AvailableBTCBalance                   prometheus.Gauge
	InvalidCheckpointCounter              prometheus.Counter
	ResentCheckpointsCounter              prometheus.Counter
	FailedResentCheckpointsCounter        prometheus.Counter
	NewSubmittedCheckpointSegmentGaugeVec *prometheus.GaugeVec
}

func newRelayerMetrics(registry *prometheus.Registry) *RelayerMetrics {
	registerer := promauto.With(registry)

	metrics := &RelayerMetrics{
		ResendIntervalSecondsGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_submitter_resend_interval",
			Help: "The intervals the submitter resends a checkpoint in seconds",
		}),
		AvailableBTCBalance: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_submitter_available_balance",
			Help: "The available balance in wallet in Satoshis",
		}),
		InvalidCheckpointCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_submitter_invalid_checkpoints",
			Help: "The number of invalid checkpoints (invalid epoch number or status)",
		}),
		ResentCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_submitter_resent_checkpoints",
			Help: "The number of resent checkpoints",
		}),
		FailedResentCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_submitter_failed_resent_checkpoints",
			Help: "The number of failed resent checkpoints",
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
		RelayerMetrics: newRelayerMetrics(registry),
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
