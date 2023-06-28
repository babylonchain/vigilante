package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ReporterMetrics struct {
	Registry                        *prometheus.Registry
	SuccessfulHeadersCounter        prometheus.Counter
	SuccessfulCheckpointsCounter    prometheus.Counter
	FailedHeadersCounter            prometheus.Counter
	FailedCheckpointsCounter        prometheus.Counter
	SecondsSinceLastHeaderGauge     prometheus.Gauge
	SecondsSinceLastCheckpointGauge prometheus.Gauge
	NewReportedHeaderGaugeVec       *prometheus.GaugeVec
	NewReportedCheckpointGaugeVec   *prometheus.GaugeVec
}

func NewReporterMetrics() *ReporterMetrics {
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)

	metrics := &ReporterMetrics{
		Registry: registry,
		SuccessfulHeadersCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_reporter_reported_headers",
			Help: "The total number of BTC headers reported to Babylon",
		}),
		SuccessfulCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_reporter_reported_checkpoints",
			Help: "The total number of BTC checkpoints reported to Babylon",
		}),
		FailedHeadersCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_reporter_failed_headers",
			Help: "The total number of failed BTC headers to Babylon",
		}),
		FailedCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_reporter_failed_checkpoints",
			Help: "The total number of failed BTC checkpoints to Babylon",
		}),
		SecondsSinceLastHeaderGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_reporter_since_last_header_seconds",
			Help: "Seconds since the last successful reported BTC header to Babylon",
		}),
		SecondsSinceLastCheckpointGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_reporter_since_last_checkpoint_seconds",
			Help: "Seconds since the last successful reported BTC checkpoint to Babylon",
		}),
		NewReportedHeaderGaugeVec: registerer.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vigilante_reporter_new_btc_header",
				Help: "The metric of a new BTC header reported to Babylon",
			},
			[]string{
				// the id of the reported BTC header
				"id",
			},
		),
		NewReportedCheckpointGaugeVec: registerer.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vigilante_reporter_new_btc_checkpoint",
				Help: "The metric of a new BTC checkpoint reported to Babylon",
			},
			[]string{
				// the epoch number of the reported checkpoint
				"epoch",
				// the BTC height of the reported checkpoint (based on the first tx)
				"height",
				// the id of the first BTC tx of the reported checkpoint
				"tx1id",
				// the id of the second BTC tx of the reported checkpoint
				"tx2id",
			},
		),
	}
	return metrics
}

func (sm *ReporterMetrics) RecordMetrics() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			// will be reset when a header/checkpoint is successfully sent
			sm.SecondsSinceLastHeaderGauge.Inc()
			sm.SecondsSinceLastCheckpointGauge.Inc()
		}
	}()

}
