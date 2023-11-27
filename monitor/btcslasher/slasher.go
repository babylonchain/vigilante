package btcslasher

import (
	"fmt"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"

	bbn "github.com/babylonchain/babylon/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/atomic"
)

const (
	txSubscriberName  = "tx-subscriber"
	messageActionName = "/babylon.finality.v1.MsgAddFinalitySig"
	evidenceEventName = "babylon.finality.v1.EventSlashedBTCValidator.evidence"
)

type BTCSlasher struct {
	// connect to BTC node
	BTCClient btcclient.BTCClient
	// BBNQuerier queries epoch info from Babylon
	BBNQuerier BabylonQueryClient

	// parameters
	netParams              *chaincfg.Params
	btcFinalizationTimeout uint64
	bsParams               *bstypes.Params

	// channel for slashed events
	evidenceChan chan *ftypes.Evidence

	metrics *metrics.SlasherMetrics

	started *atomic.Bool
	quit    chan struct{}
}

func New(
	btcClient btcclient.BTCClient,
	bbnQuerier BabylonQueryClient,
	netParams *chaincfg.Params,
	metrics *metrics.SlasherMetrics,
) (*BTCSlasher, error) {
	btccParamsResp, err := bbnQuerier.BTCCheckpointParams()
	if err != nil {
		return nil, err
	}

	bsParams, err := bbnQuerier.BTCStakingParams()
	if err != nil {
		return nil, err
	}
	return &BTCSlasher{
		BTCClient:              btcClient,
		BBNQuerier:             bbnQuerier,
		netParams:              netParams,
		bsParams:               &bsParams.Params,
		btcFinalizationTimeout: btccParamsResp.Params.CheckpointFinalizationTimeout,
		evidenceChan:           make(chan *ftypes.Evidence, 100), // TODO: parameterise buffer size
		started:                atomic.NewBool(false),
		quit:                   make(chan struct{}),
		metrics:                metrics,
	}, nil
}

func (bs *BTCSlasher) Start() {
	// ensure BTC slasher is not started yet
	if bs.started.Load() {
		log.Error("the BTC slasher is already started")
		return
	}

	// start the subscriber to slashing events
	// NOTE: at this point monitor has already started the Babylon querier routine
	// TODO: implement polling-based subscriber as well
	queryName := fmt.Sprintf("tm.event = 'Tx' AND message.action='%s'", messageActionName)
	eventChan, err := bs.BBNQuerier.Subscribe(txSubscriberName, queryName)
	if err != nil {
		log.Fatalf("failed to subscribe to %s: %v", queryName, err)
	}

	// BTC slasher has started
	bs.started.Store(true)
	log.Debugf("slasher routine has started subscribing %s", queryName)
	log.Info("the BTC slasher has started")

	// start handling incoming slashing events
	for bs.started.Load() {
		select {
		case <-bs.quit:
			// close subscriber
			if err := bs.BBNQuerier.Unsubscribe(txSubscriberName, queryName); err != nil {
				log.Errorf("failed to unsubscribe from %s with query %s: %v", txSubscriberName, queryName, err)
			}
			bs.started.Store(false)
		case evidence := <-bs.evidenceChan:
			valBTCPK := evidence.ValBtcPk
			valBTCPKHex := valBTCPK.MarshalHex()
			log.Infof("new BTC validator %s to be slashed", valBTCPKHex)
			log.Debugf("equivocation evidence of BTC validator %s: %v", valBTCPKHex, evidence)

			// extract the SK of the slashed BTC validator
			valBTCSK, err := evidence.ExtractBTCSK()
			if err != nil {
				log.Errorf("failed to extract BTC SK of the slashed BTC validator %s: %v", valBTCPKHex, err)
			}

			// slash this BTC validator's all BTC delegations
			if err := bs.SlashBTCValidator(valBTCPK, valBTCSK, false); err != nil {
				log.Errorf("failed to slash BTC validator %s: %v", valBTCPKHex, err)
			}
		case resultEvent := <-eventChan:
			if evidence := filterEvidence(&resultEvent); evidence != nil {
				log.Debugf("enqueue evidence %v to channel", evidence)
				bs.evidenceChan <- evidence
			}
		}
	}

	log.Info("the slasher is stopped")
}

// SlashBTCValidator slashes all BTC delegations under a given BTC validator
// the checkBTC option indicates whether to check the slashing tx's input is still spendable
// on Bitcoin (including mempool txs).
func (bs *BTCSlasher) SlashBTCValidator(valBTCPK *bbn.BIP340PubKey, extractedValBTCSK *btcec.PrivateKey, checkBTC bool) error {
	log.Infof("start slashing BTC validator %s", valBTCPK.MarshalHex())

	var accumulatedErrs error // we use this variable to accumulate errors

	// get all active and unbonding BTC delegations at the current BTC height
	// Some BTC delegations could be expired in Babylon's view but not expired in
	// Bitcoin's view. We will not slash such BTC delegations since they don't have
	// voting power (thus don't affect consensus) in Babylon
	activeBTCDels, unbondingBTCDels, err := bs.getAllActiveAndUnbondingBTCDelegations(valBTCPK)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegations under BTC validator %s: %w", valBTCPK.MarshalHex(), err)
	}

	// sign and submit slashing tx for each active delegation
	for _, del := range activeBTCDels {
		if err := bs.slashBTCDelegation(valBTCPK, extractedValBTCSK, del, checkBTC); err != nil {
			log.Errorf("failed to slash active BTC delegation: %v", err)
			accumulatedErrs = multierror.Append(err)
		}
	}
	// sign and submit slashing tx for each unbonding delegation
	for _, del := range unbondingBTCDels {
		if err := bs.slashBTCUndelegation(valBTCPK, extractedValBTCSK, del); err != nil {
			log.Errorf("failed to slash unbonding BTC delegation: %v", err)
			accumulatedErrs = multierror.Append(err)
		}
	}

	bs.metrics.SlashedValidatorsCounter.Inc()

	return accumulatedErrs
}

func (bs *BTCSlasher) Stop() {
	close(bs.quit)
}
