package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type UnbondingWatcherMetrics struct {
	Registry                                *prometheus.Registry
	ReportedUnbondingTransactionsCounter    prometheus.Counter
	FailedReportedUnbondingTransactions     prometheus.Counter
	NumberOfTrackedActiveDelegations        prometheus.Gauge
	DetectedUnbondingTransactionsCounter    prometheus.Counter
	DetectedNonUnbondingTransactionsCounter prometheus.Counter
}

func NewUnbondingWatcherMetrics() *UnbondingWatcherMetrics {
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)

	metrics := &UnbondingWatcherMetrics{
		Registry: registry,
		ReportedUnbondingTransactionsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "unbonding_watcher_reported_unbonding_transactions",
			Help: "The total number of unbonding transactions successfuly reported to Babylon node",
		}),
		FailedReportedUnbondingTransactions: registerer.NewCounter(prometheus.CounterOpts{
			Name: "unbonding_watcher_failed_reported_unbonding_transactions",
			Help: "The total number times reporting unbonding transactions to Babylon node failed",
		}),
		NumberOfTrackedActiveDelegations: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "unbonding_watcher_tracked_active_delegations",
			Help: "The number of active delegations tracked by unbonding watcher",
		}),
		DetectedUnbondingTransactionsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "unbonding_watcher_detected_unbonding_transactions",
			Help: "The total number of unbonding transactions detected by unbonding watcher",
		}),
		DetectedNonUnbondingTransactionsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "unbonding_watcher_detected_non_unbonding_transactions",
			Help: "The total number of non unbonding (slashing or withdrawal) transactions detected by unbonding watcher",
		}),
	}
	return metrics
}
