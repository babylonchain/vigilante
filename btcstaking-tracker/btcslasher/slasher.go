package btcslasher

import (
	"context"
	"fmt"
	"sync"
	"time"

	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"go.uber.org/zap"
)

const (
	txSubscriberName  = "tx-subscriber"
	messageActionName = "/babylon.finality.v1.MsgAddFinalitySig"
	evidenceEventName = "babylon.finality.v1.EventSlashedFinalityProvider.evidence"
)

type BTCSlasher struct {
	logger *zap.SugaredLogger

	// connect to BTC node
	BTCClient btcclient.BTCClient
	// BBNQuerier queries epoch info from Babylon
	BBNQuerier BabylonQueryClient

	// parameters
	netParams              *chaincfg.Params
	btcFinalizationTimeout uint64
	retrySleepTime         time.Duration
	maxRetrySleepTime      time.Duration

	// channel for finality signature messages, which might include
	// equivocation evidences
	finalitySigChan <-chan coretypes.ResultEvent
	// channel for SKs of slashed finality providers
	slashedFPSKChan chan *btcec.PrivateKey
	// channel for receiving the slash result of each BTC delegation
	slashResultChan chan *SlashResult

	metrics *metrics.SlasherMetrics

	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	quit      chan struct{}
}

func New(
	parentLogger *zap.Logger,
	btcClient btcclient.BTCClient,
	bbnQuerier BabylonQueryClient,
	netParams *chaincfg.Params,
	retrySleepTime time.Duration,
	maxRetrySleepTime time.Duration,
	slashedFPSKChan chan *btcec.PrivateKey,
	metrics *metrics.SlasherMetrics,
) (*BTCSlasher, error) {
	logger := parentLogger.With(zap.String("module", "slasher")).Sugar()

	return &BTCSlasher{
		logger:            logger,
		BTCClient:         btcClient,
		BBNQuerier:        bbnQuerier,
		netParams:         netParams,
		retrySleepTime:    retrySleepTime,
		maxRetrySleepTime: maxRetrySleepTime,
		slashedFPSKChan:   slashedFPSKChan,
		slashResultChan:   make(chan *SlashResult, 1000),
		quit:              make(chan struct{}),
		metrics:           metrics,
	}, nil
}

func (bs *BTCSlasher) quitContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	bs.wg.Add(1)
	go func() {
		defer cancel()
		defer bs.wg.Done()

		select {
		case <-bs.quit:

		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}

func (bs *BTCSlasher) LoadParams() error {
	if bs.btcFinalizationTimeout != 0 {
		// already loaded, skip
		return nil
	}

	btccParamsResp, err := bs.BBNQuerier.BTCCheckpointParams()
	if err != nil {
		return err
	}
	bs.btcFinalizationTimeout = btccParamsResp.Params.CheckpointFinalizationTimeout

	return nil
}

func (bs *BTCSlasher) Start() error {
	var startErr error
	bs.startOnce.Do(func() {
		// load module parameters
		if err := bs.LoadParams(); err != nil {
			startErr = err
			return
		}

		// start the subscriber to slashing events
		// NOTE: at this point monitor has already started the Babylon querier routine
		queryName := fmt.Sprintf("tm.event = 'Tx' AND message.action='%s'", messageActionName)
		bs.finalitySigChan, startErr = bs.BBNQuerier.Subscribe(txSubscriberName, queryName)
		if startErr != nil {
			return
		}
		// BTC slasher has started
		bs.logger.Debugf("slasher routine has started subscribing %s", queryName)

		// start slasher
		bs.wg.Add(2)
		go bs.equivocationTracker()
		go bs.slashingEnforcer()

		bs.logger.Info("the BTC slasher has started")
	})

	return startErr
}

// slashingEnforcer is a routine that keeps receiving finality providers
// to be slashed and slashes their BTC delegations on Bitcoin
func (bs *BTCSlasher) slashingEnforcer() {
	defer bs.wg.Done()

	bs.logger.Info("slashing enforcer has started")

	// start handling incoming slashing events
	for {
		select {
		case <-bs.quit:
			bs.logger.Debug("handle delegations loop quit")
			return
		case fpBTCSK, ok := <-bs.slashedFPSKChan:
			if !ok {
				// slasher receives the channel from outside, so its lifecycle
				// is out of slasher's control. So we need to ensure the channel
				// is not closed yet
				bs.logger.Debug("slashedFKSK channel is already closed, terminating the slashing enforcer")
				return
			}
			// slash all the BTC delegations of this finality provider
			fpBTCPKHex := bbn.NewBIP340PubKeyFromBTCPK(fpBTCSK.PubKey()).MarshalHex()
			bs.logger.Infof("slashing finality provider %s", fpBTCPKHex)

			if err := bs.SlashFinalityProvider(fpBTCSK); err != nil {
				bs.logger.Errorf("failed to slash finality provider %s: %v", fpBTCPKHex, err)
			}
		case slashRes := <-bs.slashResultChan:
			if slashRes.Err != nil {
				bs.logger.Errorf(
					"failed to slash BTC delegation with staking tx hash %s under finality provider %s: %v",
					slashRes.Del.StakingTxHex,
					slashRes.Del.FpBtcPkList[0].MarshalHex(), // TODO: work with restaking
					slashRes.Err,
				)
			} else {
				bs.logger.Infof(
					"successfully slash BTC delegation with staking tx hash %s under finality provider %s",
					slashRes.Del.StakingTxHex,
					slashRes.Del.FpBtcPkList[0].MarshalHex(), // TODO: work with restaking
				)

				// record the metrics of the slashed delegation
				bs.metrics.RecordSlashedDelegation(slashRes.Del, slashRes.SlashingTxHash.String())
			}
		}
	}
}

// equivocationTracker is a routine to track the equivocation events on Babylon,
// extract equivocating finality providers' SKs, and sen to slashing enforcer
// routine
func (bs *BTCSlasher) equivocationTracker() {
	defer bs.wg.Done()

	bs.logger.Info("equivocation tracker has started")

	// start handling incoming slashing events
	for {
		select {
		case <-bs.quit:
			bs.logger.Debug("handle delegations loop quit")
			return
		case resultEvent := <-bs.finalitySigChan:
			evidence := filterEvidence(&resultEvent)

			if evidence == nil {
				// this event does not contain equivocation evidence, skip
				continue
			}

			fpBTCPKHex := evidence.FpBtcPk.MarshalHex()
			bs.logger.Infof("new equivocating finality provider %s to be slashed", fpBTCPKHex)
			bs.logger.Debugf("found equivocation evidence of finality provider %s: %v", fpBTCPKHex, evidence)

			// extract the SK of the slashed finality provider
			fpBTCSK, err := evidence.ExtractBTCSK()
			if err != nil {
				bs.logger.Errorf("failed to extract BTC SK of the slashed finality provider %s: %v", fpBTCPKHex, err)
				continue
			}

			bs.slashedFPSKChan <- fpBTCSK
		}
	}
}

// SlashFinalityProvider slashes all BTC delegations under a given finality provider
// the checkBTC option indicates whether to check the slashing tx's input is still spendable
// on Bitcoin (including mempool txs).
func (bs *BTCSlasher) SlashFinalityProvider(extractedfpBTCSK *btcec.PrivateKey) error {
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(extractedfpBTCSK.PubKey())
	bs.logger.Infof("start slashing finality provider %s", fpBTCPK.MarshalHex())

	// get all active and unbonded BTC delegations at the current BTC height
	// Some BTC delegations could be expired in Babylon's view but not expired in
	// Bitcoin's view. We will not slash such BTC delegations since they don't have
	// voting power (thus don't affect consensus) in Babylon
	activeBTCDels, unbondedBTCDels, err := bs.getAllActiveAndUnbondedBTCDelegations(fpBTCPK)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegations under finality provider %s: %w", fpBTCPK.MarshalHex(), err)
	}

	// try to slash both staking and unbonding txs for each BTC delegation
	// sign and submit slashing tx for each active delegation
	// TODO: use semaphore to prevent spamming BTC node
	for _, del := range activeBTCDels {
		bs.wg.Add(1)
		go func(d *bstypes.BTCDelegationResponse) {
			defer bs.wg.Done()
			bs.slashBTCDelegation(fpBTCPK, extractedfpBTCSK, d)
		}(del)
	}
	// sign and submit slashing tx for each unbonded delegation
	// TODO: use semaphore to prevent spamming BTC node
	for _, del := range unbondedBTCDels {
		bs.wg.Add(1)
		go func(d *bstypes.BTCDelegationResponse) {
			defer bs.wg.Done()
			bs.slashBTCDelegation(fpBTCPK, extractedfpBTCSK, d)
		}(del)
	}

	bs.metrics.SlashedFinalityProvidersCounter.Inc()

	return nil
}

func (bs *BTCSlasher) WaitForShutdown() {
	bs.wg.Wait()
}

func (bs *BTCSlasher) Stop() error {
	var stopErr error
	bs.stopOnce.Do(func() {
		bs.logger.Info("stopping slasher")

		// close subscriber
		if err := bs.BBNQuerier.UnsubscribeAll(txSubscriberName); err != nil {
			bs.logger.Errorf("failed to unsubscribe from %s: %v", txSubscriberName, err)
		}

		// notify all subroutines
		close(bs.quit)
		bs.wg.Wait()

		bs.logger.Info("stopped slasher")
	})
	return stopErr
}
