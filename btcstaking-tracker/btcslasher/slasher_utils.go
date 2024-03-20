package btcslasher

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/avast/retry-go/v4"
	"github.com/babylonchain/babylon/btcstaking"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/utils"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
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
	Del            *bstypes.BTCDelegationResponse
	SlashingTxHash *chainhash.Hash
	Err            error
}

func (bs *BTCSlasher) slashBTCDelegation(
	fpBTCPK *bbn.BIP340PubKey,
	extractedfpBTCSK *btcec.PrivateKey,
	del *bstypes.BTCDelegationResponse,
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
	del *bstypes.BTCDelegationResponse,
	isUnbondingSlashingTx bool,
) (*chainhash.Hash, error) {
	var (
		err     error
		slashTx *bstypes.BTCSlashingTx
	)
	// check if the slashing tx is known on Bitcoin
	if isUnbondingSlashingTx {
		slashTx, err = bstypes.NewBTCSlashingTxFromHex(del.UndelegationResponse.SlashingTxHex)
		if err != nil {
			return nil, err
		}
	} else {
		slashTx, err = bstypes.NewBTCSlashingTxFromHex(del.SlashingTxHex)
		if err != nil {
			return nil, err
		}
	}

	txHash := slashTx.MustGetTxHash()
	if bs.isTxSubmittedToBitcoin(txHash) {
		// already submitted to Bitcoin, skip
		return txHash, nil
	}

	// check if the staking/unbonding tx's output is indeed spendable
	// TODO: use bbn.GetOutputIdxInBTCTx
	var spendable bool
	if isUnbondingSlashingTx {
		ubondingTx, errDecode := hex.DecodeString(del.UndelegationResponse.UnbondingTxHex)
		if errDecode != nil {
			return nil, errDecode
		}
		spendable, err = bs.isTaprootOutputSpendable(ubondingTx, 0)
	} else {
		stakingTx, errDecode := hex.DecodeString(del.StakingTxHex)
		if errDecode != nil {
			return nil, errDecode
		}
		spendable, err = bs.isTaprootOutputSpendable(stakingTx, del.StakingOutputIdx)
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
		slashingMsgTxWithWitness, err = BuildUnbondingSlashingTxWithWitness(del, bs.bsParams, bs.netParams, extractedfpBTCSK)
	} else {
		slashingMsgTxWithWitness, err = BuildSlashingTxWithWitness(del, bs.bsParams, bs.netParams, extractedfpBTCSK)
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

// BuildUnbondingSlashingTxWithWitness returns the unbonding slashing tx.
func BuildUnbondingSlashingTxWithWitness(
	d *bstypes.BTCDelegationResponse,
	bsParams *bstypes.Params,
	btcNet *chaincfg.Params,
	fpSK *btcec.PrivateKey,
) (*wire.MsgTx, error) {
	unbondingMsgTx, _, err := bbn.NewBTCTxFromHex(d.UndelegationResponse.UnbondingTxHex)
	if err != nil {
		return nil, fmt.Errorf("failed to convert a Babylon unbonding tx to wire.MsgTx: %w", err)
	}

	fpBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(d.FpBtcPkList)
	if err != nil {
		return nil, fmt.Errorf("failed to convert finality provider pks to BTC pks: %v", err)
	}

	covenantBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	if err != nil {
		return nil, fmt.Errorf("failed to convert covenant pks to BTC pks: %v", err)
	}

	// get unbonding info
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		d.BtcPk.MustToBTCPK(),
		fpBtcPkList,
		covenantBtcPkList,
		bsParams.CovenantQuorum,
		uint16(d.UnbondingTime),
		btcutil.Amount(unbondingMsgTx.TxOut[0].Value),
		btcNet,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create BTC unbonding info: %v", err)
	}
	slashingSpendInfo, err := unbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, fmt.Errorf("could not get unbonding slashing spend info: %v", err)
	}

	// get the list of covenant signatures encrypted by the given finality provider's PK
	fpPK := fpSK.PubKey()
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpPK)
	fpIdx, err := findFPIdx(fpBTCPK, d.FpBtcPkList)
	if err != nil {
		return nil, err
	}

	// fpIdx, err := findFPIdxInWitness(fpSK, d.FpBtcPkList)
	// if err != nil {
	// 	return nil, err
	// }

	covAdaptorSigs, err := bstypes.GetOrderedCovenantSignatures(fpIdx, d.UndelegationResponse.CovenantSlashingSigs, bsParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get ordered covenant adaptor signatures: %w", err)
	}

	delSlashingSig, err := bbn.NewBIP340SignatureFromHex(d.UndelegationResponse.DelegatorSlashingSigHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Delegator slashing signature: %w", err)
	}

	slashTx, err := bstypes.NewBTCSlashingTxFromHex(d.UndelegationResponse.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	// assemble witness for unbonding slashing tx
	slashingMsgTxWithWitness, err := slashTx.BuildSlashingTxWithWitness(
		fpSK,
		d.FpBtcPkList,
		unbondingMsgTx,
		0,
		delSlashingSig,
		covAdaptorSigs,
		slashingSpendInfo,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to build witness for unbonding BTC delegation %s under finality provider %s: %v",
			d.BtcPk.MarshalHex(),
			bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey()).MarshalHex(),
			err,
		)
	}

	return slashingMsgTxWithWitness, nil
}

// findFPIdx returns the index of the given finality provider
// among all restaked finality providers
func findFPIdx(fpBTCPK *bbn.BIP340PubKey, fpBtcPkList []bbn.BIP340PubKey) (int, error) {
	sortedFPBTCPKList := bbn.SortBIP340PKs(fpBtcPkList)
	for i, pk := range sortedFPBTCPKList {
		if pk.Equals(fpBTCPK) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("the given finality provider's PK is not found in the BTC delegation")
}

func BuildSlashingTxWithWitness(
	d *bstypes.BTCDelegationResponse,
	bsParams *bstypes.Params,
	btcNet *chaincfg.Params,
	fpSK *btcec.PrivateKey,
) (*wire.MsgTx, error) {
	stakingMsgTx, _, err := bbn.NewBTCTxFromHex(d.StakingTxHex)
	if err != nil {
		return nil, fmt.Errorf("failed to convert a Babylon staking tx to wire.MsgTx: %w", err)
	}

	fpBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(d.FpBtcPkList)
	if err != nil {
		return nil, fmt.Errorf("failed to convert finality provider pks to BTC pks: %v", err)
	}

	covenantBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	if err != nil {
		return nil, fmt.Errorf("failed to convert covenant pks to BTC pks: %v", err)
	}

	// get staking info
	stakingInfo, err := btcstaking.BuildStakingInfo(
		d.BtcPk.MustToBTCPK(),
		fpBtcPkList,
		covenantBtcPkList,
		bsParams.CovenantQuorum,
		uint16(d.EndHeight-d.StartHeight),
		btcutil.Amount(d.TotalSat),
		btcNet,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create BTC staking info: %v", err)
	}
	slashingSpendInfo, err := stakingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, fmt.Errorf("could not get slashing spend info: %v", err)
	}

	// get the list of covenant signatures encrypted by the given finality provider's PK
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey())
	sortedFPBTCPKList := bbn.SortBIP340PKs(d.FpBtcPkList)
	fpIdx, err := findFPIdx(fpBTCPK, sortedFPBTCPKList)
	if err != nil {
		return nil, err
	}

	covAdaptorSigs, err := bstypes.GetOrderedCovenantSignatures(fpIdx, d.CovenantSigs, bsParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get ordered covenant adaptor signatures: %w", err)
	}

	delSigSlash, err := bbn.NewBIP340SignatureFromHex(d.DelegatorSlashSigHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Delegator slashing signature: %w", err)
	}

	slashTx, err := bstypes.NewBTCSlashingTxFromHex(d.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	// assemble witness for slashing tx
	slashingMsgTxWithWitness, err := slashTx.BuildSlashingTxWithWitness(
		fpSK,
		sortedFPBTCPKList,
		stakingMsgTx,
		d.StakingOutputIdx,
		delSigSlash,
		covAdaptorSigs,
		slashingSpendInfo,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to build witness for BTC delegation of %s under finality provider %s: %v",
			d.BtcPk.MarshalHex(),
			bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey()).MarshalHex(),
			err,
		)
	}

	return slashingMsgTxWithWitness, nil
}

// BTC slasher will try to slash via staking path for active BTC delegations,
// and slash via unbonding path for unbonded delegations.
//
// An unbonded BTC delegation in Babylon's view might still
// have an non-expired timelock in unbonding tx.
func (bs *BTCSlasher) getAllActiveAndUnbondedBTCDelegations(
	fpBTCPK *bbn.BIP340PubKey,
) (activeDels, unbondedDels []*bstypes.BTCDelegationResponse, err error) {
	// wValue := bs.btcFinalizationTimeout
	activeDels, unbondedDels = make([]*bstypes.BTCDelegationResponse, 0), make([]*bstypes.BTCDelegationResponse, 0)

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
				if del.Active {
					// avoid using del which changes over the iterations
					activeDels = append(activeDels, dels.Dels[i])
				}
				if strings.EqualFold(del.StatusDesc, bstypes.BTCDelegationStatus_UNBONDED.String()) &&
					len(del.UndelegationResponse.CovenantSlashingSigs) >= int(bs.bsParams.CovenantQuorum) &&
					len(del.UndelegationResponse.DelegatorUnbondingSigHex) > 0 {
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
