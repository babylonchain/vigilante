package btcstaking_tracker

import (
	"fmt"
	"sync"

	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/btcstaking-tracker/atomicslasher"
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
	// btcSlasher monitors slashing events in BTC staking protocol,
	// and slashes BTC delegations under each equivocating finality provider
	// by signing and submitting their slashing txs
	btcSlasher IBTCSlasher
	// atomicSlasher monitors selective slashing offences where a finality
	// provider maliciously signs and submits the slashing tx of a BTC delegation.
	// Upon such a selective slashing offence the atomic slasher routine will
	// extract the finality provider's SK and send it to the BTC slasher routine
	// to slash.
	atomicSlasher IAtomicSlasher

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
	commonCfg *config.CommonConfig,
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
		commonCfg.RetrySleepTime,
		commonCfg.MaxRetrySleepTime,
		slashedFPSKChan,
		metrics.SlasherMetrics,
	)
	if err != nil {
		parentLogger.Fatal("failed to create BTC slasher", zap.Error(err))
	}

	// atomic slasher routine
	atomicSlasher := atomicslasher.New(
		cfg,
		logger,
		commonCfg.RetrySleepTime,
		commonCfg.MaxRetrySleepTime,
		btcClient,
		btcNotifier,
		bbnClient,
		slashedFPSKChan,
		metrics.AtomicSlasherMetrics,
	)

	return &BTCStakingTracker{
		cfg:              cfg,
		logger:           logger.Sugar(),
		btcClient:        btcClient,
		btcNotifier:      btcNotifier,
		bbnClient:        bbnClient,
		btcSlasher:       btcSlasher,
		atomicSlasher:    atomicSlasher,
		unbondingWatcher: watcher,
		slashedFPSKChan:  slashedFPSKChan,
		metrics:          metrics,
		quit:             make(chan struct{}),
	}
}

// Bootstrap initialises the BTC staking tracker. At the moment, only BTC
// slasher needs to be bootstrapped, in which BTC slasher checks if there is
// any previous evidence whose slashing tx is not submitted to Bitcoin yet
func (tracker *BTCStakingTracker) Bootstrap(startHeight uint64) error {
	// bootstrap BTC slasher
	if err := tracker.btcSlasher.Bootstrap(startHeight); err != nil {
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
		if err := tracker.btcSlasher.Start(); err != nil {
			startErr = err
			return
		}
		if err := tracker.atomicSlasher.Start(); err != nil {
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
		if err := tracker.btcSlasher.Stop(); err != nil {
			stopErr = err
			return
		}
		if err := tracker.atomicSlasher.Stop(); err != nil {
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
