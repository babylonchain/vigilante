package btcslasher

import (
	"fmt"

	bbn "github.com/babylonchain/babylon/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/atomic"
)

const (
	slasherSubscriberName    = "slashed-btc-validator-subscriber"
	slashEventName           = "EventSlashedBTCValidator"
	slasherChannelBufferSize = 100 // TODO: parameterise
)

type BTCSlasher struct {
	// connect to BTC node
	BTCClient btcclient.BTCClient
	// BBNQuerier queries epoch info from Babylon
	BBNQuerier BabylonQueryClient

	netParams              *chaincfg.Params
	btcFinalizationTimeout uint64

	started *atomic.Bool
	quit    chan struct{}
}

func New(btcClient btcclient.BTCClient, bbnQuerier BabylonQueryClient, netParams *chaincfg.Params) (*BTCSlasher, error) {
	btccParamsResp, err := bbnQuerier.BTCCheckpointParams()
	if err != nil {
		return nil, err
	}

	return &BTCSlasher{
		BTCClient:              btcClient,
		BBNQuerier:             bbnQuerier,
		netParams:              netParams,
		btcFinalizationTimeout: btccParamsResp.Params.CheckpointFinalizationTimeout,
		started:                atomic.NewBool(false),
		quit:                   make(chan struct{}),
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
	// TODO: query condition with height constraint
	// TODO: investigate subscriber behaviours and decide whether to pull or subscribe
	slasherQueryName := tmtypes.QueryForEvent(slashEventName).String()
	eventChan, err := bs.BBNQuerier.Subscribe(slasherSubscriberName, slasherQueryName, slasherChannelBufferSize)
	if err != nil {
		log.Fatalf("failed to subscribe to %s: %v", slashEventName, err)
	}

	// BTC slasher has started
	bs.started.Store(true)
	log.Debugf("slasher routine has started subscribing %s", slashEventName)
	log.Info("the BTC scanner has started")

	// start handling incoming slashing events
	for bs.started.Load() {
		select {
		case <-bs.quit:
			// close subscriber
			if err := bs.BBNQuerier.Unsubscribe(slasherSubscriberName, slasherQueryName); err != nil {
				log.Errorf("failed to unsubscribe from %s with query %s: %v", slasherSubscriberName, slasherQueryName, err)
			}
			bs.started.Store(false)
		case event := <-eventChan:
			slashEvent, ok := event.Data.(ftypes.EventSlashedBTCValidator)
			if !ok {
				// this has to be a programming error in Babylon side
				log.Errorf("failed to cast event with correct name to %s", slashEventName)
				continue
			}

			valBTCPK := slashEvent.ValBtcPk
			valBTCPKHex := valBTCPK.MarshalHex()
			log.Infof("new BTC validator %s to be slashed", valBTCPKHex)
			log.Debugf("equivocation evidence of BTC validator %s: %v", valBTCPKHex, slashEvent.Evidence)

			// extract the SK of the slashed BTC validator
			valBTCSKBytes := slashEvent.ExtractedBtcSk
			valBTCSK, _ := btcec.PrivKeyFromBytes(valBTCSKBytes)

			// slash this BTC validator's all BTC delegations
			if err := bs.SlashBTCValidator(valBTCPK, valBTCSK); err != nil {
				log.Errorf("failed to slash BTC validator %s: %v", valBTCPKHex, err)
			}
		}
	}

	log.Info("the slasher is stopped")
}

func (bs *BTCSlasher) SlashBTCValidator(valBTCPK *bbn.BIP340PubKey, extractedValBTCSK *btcec.PrivateKey) error {
	var accumulatedErrs error // we use this variable to accumulate errors

	// get all slashable BTC delegations whose timelock is not expired yet
	// TODO: some BTC delegation could be expired in Babylon's view and is not expired in
	// Bitcoin's view. We still slash such BTC delegations for now, but will revisit the design later
	slashableBTCDels, err := bs.getAllSlashableBTCDelegations(valBTCPK)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegations under BTC validator %s: %w", valBTCPK.MarshalHex(), err)
	}

	// sign and submit slashing tx for each of this BTC validator's delegation
	for _, del := range slashableBTCDels {
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
		// TODO: use SendRawTransactionAsync
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

		// TODO: wait for k-deep to ensure slashing tx is included
	}

	return accumulatedErrs
}

func (bs *BTCSlasher) Stop() {
	close(bs.quit)
}
