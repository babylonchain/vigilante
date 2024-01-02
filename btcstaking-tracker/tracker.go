package btcstaking_tracker

import (
	"fmt"
	"sync"

	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/btcstaking-tracker/btcslasher"
	uw "github.com/babylonchain/vigilante/btcstaking-tracker/unbondingwatcher"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/btcec/v2"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"go.uber.org/zap"
)

type BTCStakingTracker struct {
	cfg    *config.BTCStakingTrackerConfig
	logger *zap.SugaredLogger

	btcClient   btcclient.BTCClient // TODO: limit the scope
	btcNotifier notifier.ChainNotifier
	// TODO: Ultimately all requests to babylon should go through some kind of semaphore
	// to avoid spamming babylon with requests
	bbnClient *bbnclient.Client

	// unbondingWatcher monitors early unbonding transactions on Bitcoin
	// and reports unbonding BTC delegations back to Babylon
	unbondingWatcher *uw.UnbondingWatcher
	// BTCSlasher monitors slashing events in BTC staking protocol,
	// and slashes BTC delegations under each equivocating finality provider
	// by signing and submitting their slashing txs
	BTCSlasher IBTCSlasher

	// slashedFPSKChan is a channel that contains BTC SKs of slashed finality
	// providers. BTC slasher produces SKs of equivocating finality providers
	// to the channel. Atomic slasher produces SKs of finality providers who
	// selective slash BTC delegations to the channel. Slashing enforcer routine
	// in the BTC slasher consumes the channel.
	slashedFPSKChan chan *btcec.PrivateKey

	metrics *metrics.BTCStakingTrackerMetrics

	startOnce sync.Once
	stopOnce  sync.Once
	quit      chan struct{}
}

func NewBTCSTakingTracker(
	btcClient btcclient.BTCClient,
	btcNotifier notifier.ChainNotifier,
	bbnClient *bbnclient.Client,
	cfg *config.BTCStakingTrackerConfig,
	parentLogger *zap.Logger,
	metrics *metrics.BTCStakingTrackerMetrics,
) *BTCStakingTracker {
	logger := parentLogger.With(zap.String("module", "btcstaking-tracker"))

	// watcher routine
	babylonAdapter := uw.NewBabylonClientAdapter(bbnClient)
	watcher := uw.NewUnbondingWatcher(btcNotifier, babylonAdapter, cfg, logger, metrics.UnbondingWatcherMetrics)

	slashedFPSKChan := make(chan *btcec.PrivateKey, 100) // TODO: parameterise buffer size

	// BTC slasher routine
	// NOTE: To make subscriber in slasher work, the underlying RPC client
	// has to be kept running with a websocket connection
	bbnQueryClient := bbnClient.QueryClient
	btcParams, err := netparams.GetBTCParams(cfg.BTCNetParams)
	if err != nil {
		parentLogger.Fatal("failed to get BTC parameter", zap.Error(err))
	}
	btcSlasher, err := btcslasher.New(
		logger,
		btcClient,
		bbnQueryClient,
		btcParams,
		slashedFPSKChan,
		metrics.SlasherMetrics,
	)
	if err != nil {
		panic(fmt.Errorf("failed to create BTC slasher: %w", err))
	}

	return &BTCStakingTracker{
		cfg:              cfg,
		logger:           logger.Sugar(),
		btcClient:        btcClient,
		btcNotifier:      btcNotifier,
		bbnClient:        bbnClient,
		BTCSlasher:       btcSlasher,
		unbondingWatcher: watcher,
		slashedFPSKChan:  slashedFPSKChan,
		metrics:          metrics,
		quit:             make(chan struct{}),
	}
}

// Bootstrap initialises the monitor. At the moment, only BTC slasher
// needs to be bootstrapped, in which BTC slasher checks if there is
// any previous evidence whose slashing tx is not submitted to Bitcoin yet
func (tracker *BTCStakingTracker) Bootstrap(startHeight uint64) error {
	// bootstrap slasher
	if err := tracker.BTCSlasher.Bootstrap(startHeight); err != nil {
		return fmt.Errorf("failed to bootstrap BTC staking tracker: %w", err)
	}
	return nil
}

func (tracker *BTCStakingTracker) Start() error {
	var startErr error
	tracker.startOnce.Do(func() {
		tracker.logger.Info("starting BTC staking tracker")

		if err := tracker.unbondingWatcher.Start(); err != nil {
			startErr = err
			return
		}
		if err := tracker.BTCSlasher.Start(); err != nil {
			startErr = err
			return
		}

		tracker.logger.Info("BTC staking tracker started")
	})

	return startErr
}

func (tracker *BTCStakingTracker) Stop() error {
	var stopErr error
	tracker.stopOnce.Do(func() {
		tracker.logger.Info("stopping BTC staking tracker")

		if err := tracker.unbondingWatcher.Stop(); err != nil {
			stopErr = err
			return
		}
		if err := tracker.BTCSlasher.Stop(); err != nil {
			stopErr = err
			return
		}
		if err := tracker.bbnClient.Stop(); err != nil {
			stopErr = err
			return
		}

		close(tracker.slashedFPSKChan)
		close(tracker.quit)

		tracker.logger.Info("stopped BTC staking tracker")
	})
	return stopErr
}
