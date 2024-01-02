package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type BTCStakingTrackerMetrics struct {
	Registry *prometheus.Registry
	*UnbondingWatcherMetrics
	*SlasherMetrics
}

type UnbondingWatcherMetrics struct {
	Registry                                *prometheus.Registry
	ReportedUnbondingTransactionsCounter    prometheus.Counter
	FailedReportedUnbondingTransactions     prometheus.Counter
	NumberOfTrackedActiveDelegations        prometheus.Gauge
	DetectedUnbondingTransactionsCounter    prometheus.Counter
	DetectedNonUnbondingTransactionsCounter prometheus.Counter
}

func newUnbondingWatcherMetrics(registry *prometheus.Registry) *UnbondingWatcherMetrics {
	registerer := promauto.With(registry)

	uwMetrics := &UnbondingWatcherMetrics{
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

	return uwMetrics
}

func NewBTCStakingTrackerMetrics() *BTCStakingTrackerMetrics {
	registry := prometheus.NewRegistry()
	uwMetrics := newUnbondingWatcherMetrics(registry)
	slasherMetrics := newSlasherMetrics(registry)

	return &BTCStakingTrackerMetrics{registry, uwMetrics, slasherMetrics}
}

type SlasherMetrics struct {
	SlashedDelegationGaugeVec       *prometheus.GaugeVec
	SlashedFinalityProvidersCounter prometheus.Counter
	SlashedDelegationsCounter       prometheus.Counter
	SlashedSatsCounter              prometheus.Counter
}

func newSlasherMetrics(registry *prometheus.Registry) *SlasherMetrics {
	registerer := promauto.With(registry)

	metrics := &SlasherMetrics{
		SlashedFinalityProvidersCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_slashed_finality_providers",
			Help: "The number of slashed finality providers",
		}),
		SlashedDelegationsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_slashed_delegations",
			Help: "The number of slashed delegations",
		}),
		SlashedSatsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_slashed_sats",
			Help: "The amount of slashed funds in Satoshi",
		}),
		SlashedDelegationGaugeVec: registerer.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vigilante_monitor_new_slashed_delegation",
				Help: "The metric of a newly slashed delegation",
			},
			[]string{
				// del_babylon_pk is the Babylon secp256k1 PK of this BTC delegation in hex string
				"del_babylon_pk",
				// del_btc_pk is the Bitcoin secp256k1 PK of this BTC delegation in hex string
				"del_btc_pk",
				// val_btc_pk is the Bitcoin secp256k1 PK of the finality provider that
				// this BTC delegation delegates to, in hex string
				"val_btc_pk",
				// start_height is the start BTC height of the BTC delegation
				// it is the start BTC height of the timelock
				"start_height",
				// end_height is the end height of the BTC delegation
				// it is the end BTC height of the timelock - w
				"end_height",
				// total_sat is the total amount of BTC stakes in this delegation
				// quantified in satoshi
				"total_sat",
				// slashing_tx_hash is the hash of the slashing tx
				"slashing_tx_hash",
			},
		),
	}

	return metrics
}
