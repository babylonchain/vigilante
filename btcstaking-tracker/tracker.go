package btcstaking_tracker

import (
	"sync"

	bsttypes "github.com/babylonchain/vigilante/btcstaking-tracker/types"
	uw "github.com/babylonchain/vigilante/btcstaking-tracker/unbondingwatcher"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"go.uber.org/zap"
)

type BTCStakingTracker struct {
	cfg    *config.BTCStakingTrackerConfig
	logger *zap.SugaredLogger

	btcNotifier notifier.ChainNotifier
	// TODO: Ultimately all requests to babylon should go through some kind of semaphore
	// to avoid spamming babylon with requests
	babylonNodeAdapter bsttypes.BabylonNodeAdapter

	unbondingWatcher *uw.UnbondingWatcher

	metrics *metrics.BTCStakingTrackerMetrics

	startOnce sync.Once
	stopOnce  sync.Once
	quit      chan struct{}
}

func NewBTCSTakingTracker(
	btcNotifier notifier.ChainNotifier,
	babylonNodeAdapter bsttypes.BabylonNodeAdapter,
	cfg *config.BTCStakingTrackerConfig,
	parentLogger *zap.Logger,
	metrics *metrics.BTCStakingTrackerMetrics,
) *BTCStakingTracker {
	logger := parentLogger.With(zap.String("module", "btcstaking-tracker"))
	watcher := uw.NewUnbondingWatcher(btcNotifier, babylonNodeAdapter, cfg, logger, metrics.UnbondingWatcherMetrics)
	return &BTCStakingTracker{
		cfg:                cfg,
		logger:             logger.Sugar(),
		btcNotifier:        btcNotifier,
		babylonNodeAdapter: babylonNodeAdapter,
		unbondingWatcher:   watcher,
		metrics:            metrics,
		quit:               make(chan struct{}),
	}
}

func (tracker *BTCStakingTracker) Start() error {
	var startErr error
	tracker.startOnce.Do(func() {
		tracker.logger.Info("starting BTC staking tracker")

		if err := tracker.unbondingWatcher.Start(); err != nil {
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
		close(tracker.quit)
		if err := tracker.unbondingWatcher.Stop(); err != nil {
			stopErr = err
			return
		}
		tracker.logger.Info("stopping BTC staking tracker")
	})
	return stopErr
}
