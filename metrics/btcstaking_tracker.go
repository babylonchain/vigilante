package metrics

import (
	"github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type BTCStakingTrackerMetrics struct {
	Registry *prometheus.Registry
	*UnbondingWatcherMetrics
	*SlasherMetrics
	*AtomicSlasherMetrics
}

func NewBTCStakingTrackerMetrics() *BTCStakingTrackerMetrics {
	registry := prometheus.NewRegistry()
	uwMetrics := newUnbondingWatcherMetrics(registry)
	slasherMetrics := newSlasherMetrics(registry)
	atomicSlasherMetrics := newAtomicSlasherMetrics(registry)

	return &BTCStakingTrackerMetrics{registry, uwMetrics, slasherMetrics, atomicSlasherMetrics}
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
			Name: "slasher_slashed_finality_providers",
			Help: "The number of slashed finality providers",
		}),
		SlashedDelegationsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "slasher_slashed_delegations",
			Help: "The number of slashed delegations",
		}),
		SlashedSatsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "slasher_slashed_sats",
			Help: "The amount of slashed funds in Satoshi",
		}),
		SlashedDelegationGaugeVec: registerer.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "slasher_new_slashed_delegation",
				Help: "The metric of a newly slashed delegation",
			},
			[]string{
				// del_btc_pk is the Bitcoin secp256k1 PK of this BTC delegation in hex string
				"del_btc_pk",
				// fp_btc_pk is the Bitcoin secp256k1 PK of the finality provider that
				// this BTC delegation delegates to, in hex string
				"fp_btc_pk",
			},
		),
	}

	return metrics
}

func (sm *SlasherMetrics) RecordSlashedDelegation(del *types.BTCDelegationResponse, txHashStr string) {
	// refresh time of the slashed delegation gauge for each (fp, del) pair
	for _, pk := range del.FpBtcPkList {
		sm.SlashedDelegationGaugeVec.WithLabelValues(
			del.BtcPk.MarshalHex(),
			pk.MarshalHex(),
		).SetToCurrentTime()
	}

	// increment slashed Satoshis and slashed delegations
	sm.SlashedSatsCounter.Add(float64(del.TotalSat))
	sm.SlashedDelegationsCounter.Inc()
}

type AtomicSlasherMetrics struct {
	Registry                   *prometheus.Registry
	TrackedBTCDelegationsGauge prometheus.Gauge
}

func newAtomicSlasherMetrics(registry *prometheus.Registry) *AtomicSlasherMetrics {
	registerer := promauto.With(registry)

	asMetrics := &AtomicSlasherMetrics{
		Registry: registry,
		TrackedBTCDelegationsGauge: registerer.NewGauge(prometheus.GaugeOpts{
			Name: "atomic_slasher_tracked_delegations_gauge",
			Help: "The number of BTC delegations the atomic slasher routine is tracking",
		}),
	}

	return asMetrics
}
