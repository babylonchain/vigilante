package btcslasher

import (
	"fmt"
	"strings"

	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/gogo/protobuf/jsonpb"
)

const (
	defaultPaginationLimit = 100
)

func (bs *BTCSlasher) slashBTCDelegation(fpBTCPK *bbn.BIP340PubKey, extractedfpBTCSK *btcec.PrivateKey,
	del *bstypes.BTCDelegation, checkBTC bool) error {
	if checkBTC {
		slashable, err := bs.isTaprootOutputSpendable(del.StakingTx, del.StakingOutputIdx)
		if err != nil {
			// Warning: this can only be an error in Bitcoin side
			return fmt.Errorf(
				"failed to check if BTC delegation %s under finality provider %s is slashable: %v",
				del.BtcPk.MarshalHex(),
				fpBTCPK.MarshalHex(),
				err,
			)
		}
		// skip unslashable BTC delegation
		if !slashable {
			return nil
		}
	}

	// assemble witness for slashing tx
	slashingMsgTxWithWitness, err := del.BuildSlashingTxWithWitness(bs.bsParams, bs.netParams, extractedfpBTCSK)
	if err != nil {
		// Warning: this can only be a programming error in Babylon side
		return fmt.Errorf(
			"failed to build witness for BTC delegation %s under finality provider %s: %v",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
			err,
		)
	}
	bs.logger.Debugf(
		"signed and assembled witness for slashing tx of BTC delegation %s under finality provider %s",
		del.BtcPk.MarshalHex(),
		fpBTCPK.MarshalHex(),
	)

	// submit slashing tx
	txHash, err := bs.BTCClient.SendRawTransaction(slashingMsgTxWithWitness, true)
	if err != nil {
		return fmt.Errorf(
			"failed to submit slashing tx of BTC delegation %s under finality provider %s to Bitcoin: %v",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
			err,
		)
	}
	bs.logger.Infof(
		"successfully submitted slashing tx (txHash: %s) for BTC delegation %s under finality provider %s",
		txHash.String(),
		del.BtcPk.MarshalHex(),
		fpBTCPK.MarshalHex(),
	)

	// record the metrics of the slashed delegation
	bs.metrics.RecordSlashedDelegation(del, txHash.String())

	// TODO: wait for k-deep to ensure slashing tx is included

	return nil
}

func (bs *BTCSlasher) slashBTCUndelegation(fpBTCPK *bbn.BIP340PubKey, extractedfpBTCSK *btcec.PrivateKey, del *bstypes.BTCDelegation) error {
	// check if the unbonding tx's output is indeed spendable
	// Unbonding transaction always has one output so outIdx is always 0
	spendable, err := bs.isTaprootOutputSpendable(del.BtcUndelegation.UnbondingTx, 0)
	if err != nil {
		// Warning: this can only be an error in Bitcoin side
		return fmt.Errorf(
			"failed to check if unbonding BTC delegation %s under finality provider %s is slashable: %v",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
			err,
		)
	}
	// this might mean unbonding BTC delegation did not honestly submit unbonding tx to Bitcoin
	// try to slash BTC delegation instead
	if !spendable {
		bs.logger.Warnf(
			"the unbonding BTC delegation %s under finality provider %s did not honestly submit its unbonding tx to Bitcoin. Try to slash via its staking tx instead",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
		)
		return bs.slashBTCDelegation(fpBTCPK, extractedfpBTCSK, del, true)
	}

	// assemble witness for slashing tx
	slashingMsgTxWithWitness, err := del.BuildUnbondingSlashingTxWithWitness(bs.bsParams, bs.netParams, extractedfpBTCSK)
	if err != nil {
		// Warning: this can only be a programming error in Babylon side
		return fmt.Errorf(
			"failed to build witness for unbonded BTC delegation %s under finality provider %s: %v",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
			err,
		)
	}
	bs.logger.Debugf(
		"signed and assembled witness for slashing tx of unbonded BTC delegation %s under finality provider %s",
		del.BtcPk.MarshalHex(),
		fpBTCPK.MarshalHex(),
	)

	// submit slashing tx
	txHash, err := bs.BTCClient.SendRawTransaction(slashingMsgTxWithWitness, true)
	if err != nil {
		return fmt.Errorf(
			"failed to submit slashing tx of unbonded BTC delegation %s under finality provider %s to Bitcoin: %v",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
			err,
		)
	}
	bs.logger.Infof(
		"successfully submitted slashing tx (txHash: %s) for unbonded BTC delegation %s under finality provider %s",
		txHash.String(),
		del.BtcPk.MarshalHex(),
		fpBTCPK.MarshalHex(),
	)

	// record the metrics of the slashed delegation
	bs.metrics.RecordSlashedDelegation(del, txHash.String())

	// TODO: wait for k-deep to ensure slashing tx is included

	return nil
}

// BTC slasher will try to slash via staking path for active BTC delegations,
// and slash via unbonding path for unbonded delegations.
//
// An unbonded BTC delegation in Babylon's view might still
// have an non-expired timelock in unbonding tx.
func (bs *BTCSlasher) getAllActiveAndUnbondedBTCDelegations(fpBTCPK *bbn.BIP340PubKey) ([]*bstypes.BTCDelegation, []*bstypes.BTCDelegation, error) {
	wValue := bs.btcFinalizationTimeout
	activeDels := []*bstypes.BTCDelegation{}
	unbondedDels := []*bstypes.BTCDelegation{}

	// get BTC tip height
	_, btcTipHeight, err := bs.BTCClient.GetBestBlock()
	if err != nil {
		return nil, nil, err
	}

	// get all active BTC delegations
	pagination := query.PageRequest{Limit: defaultPaginationLimit}
	for {
		resp, err := bs.BBNQuerier.FinalityProviderDelegations(fpBTCPK.MarshalHex(), &pagination)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get BTC delegations under finality provider %s: %w", fpBTCPK.MarshalHex(), err)
		}
		for _, dels := range resp.BtcDelegatorDelegations {
			for i, del := range dels.Dels {
				// filter out all active and unbonded BTC delegations
				// NOTE: slasher does not slash BTC delegations who
				//   - is expired in Babylon due to the timelock of <w rest blocks, OR
				//   - has an expired timelock but the delegator hasn't moved its stake yet
				// This is because such BTC delegations do not have voting power thus do not
				// affect Babylon's consensus.
				delStatus := del.GetStatus(btcTipHeight, wValue, bs.bsParams.CovenantQuorum)
				if delStatus == bstypes.BTCDelegationStatus_ACTIVE {
					// avoid using del which changes over the iterations
					activeDels = append(activeDels, dels.Dels[i])
				}
				if delStatus == bstypes.BTCDelegationStatus_UNBONDED &&
					del.BtcUndelegation.HasCovenantQuorumOnSlashing(bs.bsParams.CovenantQuorum) &&
					del.BtcUndelegation.DelegatorUnbondingSig != nil {
					// NOTE: Babylon considers a BTC delegation to be unbonded once it
					// receives staker signature for unbonding transaction, no matter
					// whether the unbonding tx's timelock has expired. In monitor's view we need to try to slash every
					// BTC delegation with a non-nil BTC undelegation and with jury/delegator signature on slashing tx
					// avoid using del which changes over the iterations
					unbondedDels = append(unbondedDels, dels.Dels[i])
				}
			}
		}
		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}

	return activeDels, unbondedDels, nil
}

func filterEvidence(resultEvent *coretypes.ResultEvent) *ftypes.Evidence {
	for eventName, eventData := range resultEvent.Events {
		if strings.Contains(eventName, evidenceEventName) {
			if len(eventData) > 0 {
				var evidence ftypes.Evidence
				if err := jsonpb.UnmarshalString(eventData[0], &evidence); err != nil {
					continue
				}
				return &evidence
			}
		}
	}
	return nil
}
