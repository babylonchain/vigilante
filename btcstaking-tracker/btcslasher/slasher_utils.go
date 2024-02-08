package btcslasher

import (
	"fmt"
	"strings"

	"github.com/avast/retry-go/v4"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/utils"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/hashicorp/go-multierror"
)

const (
	defaultPaginationLimit = 100
)

type SlashResult struct {
	Del            *bstypes.BTCDelegation
	SlashingTxHash *chainhash.Hash
	Err            error
}

func (bs *BTCSlasher) slashBTCDelegation(
	fpBTCPK *bbn.BIP340PubKey,
	extractedfpBTCSK *btcec.PrivateKey,
	del *bstypes.BTCDelegation,
) {
	var txHash *chainhash.Hash

	ctx, cancel := bs.quitContext()
	defer cancel()

	err := retry.Do(
		func() error {
			var accumulatedErrs error

			txHash1, err1 := bs.sendSlashingTx(fpBTCPK, extractedfpBTCSK, del, false)
			txHash2, err2 := bs.sendSlashingTx(fpBTCPK, extractedfpBTCSK, del, true)
			if err1 != nil && err2 != nil {
				// both attempts fail
				accumulatedErrs = multierror.Append(accumulatedErrs, err1, err2)
				txHash = nil
			} else if err1 == nil {
				// slashing tx is submitted successfully
				txHash = txHash1
			} else if err2 == nil {
				// unbonding slashing tx is submitted successfully
				txHash = txHash2
			}

			return accumulatedErrs
		},
		retry.Context(ctx),
		retry.Delay(bs.retrySleepTime),
		retry.MaxDelay(bs.maxRetrySleepTime),
	)

	slashRes := &SlashResult{
		Del:            del,
		SlashingTxHash: txHash,
		Err:            err,
	}
	utils.PushOrQuit[*SlashResult](bs.slashResultChan, slashRes, bs.quit)
}

func (bs *BTCSlasher) sendSlashingTx(
	fpBTCPK *bbn.BIP340PubKey,
	extractedfpBTCSK *btcec.PrivateKey,
	del *bstypes.BTCDelegation,
	isUnbondingSlashingTx bool,
) (*chainhash.Hash, error) {
	var err error

	// check if the slashing tx is known on Bitcoin
	var txHash *chainhash.Hash
	if isUnbondingSlashingTx {
		txHash = del.BtcUndelegation.SlashingTx.MustGetTxHash()
	} else {
		txHash = del.SlashingTx.MustGetTxHash()
	}
	if bs.isTxSubmittedToBitcoin(txHash) {
		// already submitted to Bitcoin, skip
		return txHash, nil
	}

	// check if the staking/unbonding tx's output is indeed spendable
	// TODO: use bbn.GetOutputIdxInBTCTx
	var spendable bool
	if isUnbondingSlashingTx {
		spendable, err = bs.isTaprootOutputSpendable(del.BtcUndelegation.UnbondingTx, 0)
	} else {
		spendable, err = bs.isTaprootOutputSpendable(del.StakingTx, del.StakingOutputIdx)
	}
	if err != nil {
		// Warning: this can only be an error in Bitcoin side
		return nil, fmt.Errorf(
			"failed to check if BTC delegation %s under finality provider %s is slashable: %v",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
			err,
		)
	}
	// this staking/unbonding tx is no longer slashable on Bitcoin
	if !spendable {
		return nil, fmt.Errorf(
			"the staking/unbonding tx of BTC delegation %s under finality provider %s is not slashable",
			del.BtcPk.MarshalHex(),
			fpBTCPK.MarshalHex(),
		)
	}

	// assemble witness for unbonding slashing tx
	var slashingMsgTxWithWitness *wire.MsgTx
	if isUnbondingSlashingTx {
		slashingMsgTxWithWitness, err = del.BuildUnbondingSlashingTxWithWitness(bs.bsParams, bs.netParams, extractedfpBTCSK)
	} else {
		slashingMsgTxWithWitness, err = del.BuildSlashingTxWithWitness(bs.bsParams, bs.netParams, extractedfpBTCSK)
	}
	if err != nil {
		// Warning: this can only be a programming error in Babylon side
		return nil, fmt.Errorf(
			"failed to build witness for BTC delegation %s under finality provider %s: %v",
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
	txHash, err = bs.BTCClient.SendRawTransaction(slashingMsgTxWithWitness, true)
	if err != nil {
		return nil, fmt.Errorf(
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

	// TODO: wait for k-deep to ensure slashing tx is included

	return txHash, nil
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
