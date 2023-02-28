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
}

func NewReporterMetrics() *ReporterMetrics {
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)

	metrics := &ReporterMetrics{
		Registry: registry,
		SuccessfulHeadersCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_reporter_reported_headers",
			Help: "The total number of reported BTC headers to Babylon",
		}),
		SuccessfulCheckpointsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_reporter_reported_checkpoints",
			Help: "The total number of reported BTC checkpoints to Babylon",
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
			Help: "Seconds since the last successful reported BTC checkpoint to Babylon",
		}),
		SecondsSinceLastCheckpointGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "vigilante_reporter_since_last_submission_seconds",
			Help: "Seconds since the last successful reported BTC checkpoint to Babylon",
		}),
	}
	return metrics
}

func (sm *ReporterMetrics) RecordMetrics() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			sm.SecondsSinceLastHeaderGauge.Inc()
			sm.SecondsSinceLastCheckpointGauge.Inc()
		}
	}()

}
