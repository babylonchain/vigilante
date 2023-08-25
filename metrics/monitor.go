package metrics

import (
	"encoding/hex"
	"strconv"

	"github.com/babylonchain/babylon/x/btcstaking/types"
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
	*SlasherMetrics
}

type SlasherMetrics struct {
	SlashedDelegationGaugeVec *prometheus.GaugeVec
	SlashedValidatorsCounter  prometheus.Counter
	SlashedDelegationsCounter prometheus.Counter
	SlashedSatsCounter        prometheus.Counter
}

func newSlasherMetrics(registry *prometheus.Registry) *SlasherMetrics {
	registerer := promauto.With(registry)

	metrics := &SlasherMetrics{
		SlashedValidatorsCounter: registerer.NewCounter(prometheus.CounterOpts{
			Name: "vigilante_monitor_slashed_validators",
			Help: "The number of slashed validators",
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
				// val_btc_pk is the Bitcoin secp256k1 PK of the BTC validator that
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
		SlasherMetrics: newSlasherMetrics(registry),
	}
	return metrics
}

func (sm *SlasherMetrics) RecordSlashedDelegation(del *types.BTCDelegation, txHashStr string) {
	sm.SlashedDelegationGaugeVec.WithLabelValues(
		hex.EncodeToString(del.BabylonPk.Key),
		del.BtcPk.MarshalHex(),
		del.ValBtcPk.MarshalHex(),
		strconv.Itoa(int(del.StartHeight)),
		strconv.Itoa(int(del.EndHeight)),
		strconv.Itoa(int(del.TotalSat)),
		txHashStr,
	).SetToCurrentTime()
	sm.SlashedSatsCounter.Add(float64(del.TotalSat))
	sm.SlashedDelegationsCounter.Inc()
}
