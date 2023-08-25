package btcslasher

import (
	"fmt"

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

	return &BTCSlasher{
		BTCClient:              btcClient,
		BBNQuerier:             bbnQuerier,
		netParams:              netParams,
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

	// TODO: bootstrap process

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
	log.Info("the BTC scanner has started")

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
			if err := bs.SlashBTCValidator(valBTCPK, valBTCSK); err != nil {
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

func (bs *BTCSlasher) SlashBTCValidator(valBTCPK *bbn.BIP340PubKey, extractedValBTCSK *btcec.PrivateKey) error {
	log.Infof("start slashing BTC validator %s", valBTCPK.MarshalHex())

	var accumulatedErrs error // we use this variable to accumulate errors

	// get all active BTC delegations
	// Some BTC delegations could be expired in Babylon's view but not expired in
	// Bitcoin's view. We will not slash such BTC delegations since they don't have
	// voting power (thus don't affect consensus) in Babylon
	activeBTCDels, err := bs.getAllActiveBTCDelegations(valBTCPK)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegations under BTC validator %s: %w", valBTCPK.MarshalHex(), err)
	}

	// sign and submit slashing tx for each of this BTC validator's active delegations
	for _, del := range activeBTCDels {
		// assemble witness for slashing tx
		slashingMsgTxWithWitness, err := bs.buildSlashingTxWithWitness(extractedValBTCSK, del)
		if err != nil {
			// Warning: this can only be a programming error in Babylon side
			log.Warnf(
				"failed to build witness for BTC delegation %s under BTC validator %s: %v",
				del.BtcPk.MarshalHex(),
				valBTCPK.MarshalHex(),
				err,
			)
			accumulatedErrs = multierror.Append(accumulatedErrs, err)
			continue
		}
		log.Debugf(
			"signed and assembled witness for slashing tx of BTC delegation %s under BTC validator %s",
			del.BtcPk.MarshalHex(),
			valBTCPK.MarshalHex(),
		)

		// submit slashing tx
		txHash, err := bs.BTCClient.SendRawTransaction(slashingMsgTxWithWitness, true)
		if err != nil {
			log.Errorf(
				"failed to submit slashing tx of BTC delegation %s under BTC validator %s to Bitcoin: %v",
				del.BtcPk.MarshalHex(),
				valBTCPK.MarshalHex(),
				err,
			)
			accumulatedErrs = multierror.Append(accumulatedErrs, err)
			continue
		}
		log.Infof(
			"successfully submitted slashing tx (txHash: %s) for BTC delegation %s under BTC validator %s",
			txHash.String(),
			del.BtcPk.MarshalHex(),
			valBTCPK.MarshalHex(),
		)

		// record the metrics of the slashed delegation
		bs.metrics.RecordSlashedDelegation(del, txHash.String())

		// TODO: wait for k-deep to ensure slashing tx is included
	}

	bs.metrics.SlashedValidatorsCounter.Inc()

	return accumulatedErrs
}

func (bs *BTCSlasher) Stop() {
	close(bs.quit)
}
