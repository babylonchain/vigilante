package btcslasher

import (
	"fmt"

	"github.com/babylonchain/babylon/btcstaking"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// isTaprootOutputSpendable checks if the taproot output of a given tx is still spendable on Bitcoin
// This function can be used to check the output of a staking tx or an undelegation tx
func (bs *BTCSlasher) isTaprootOutputSpendable(tx *bstypes.BabylonBTCTaprootTx) (bool, error) {
	txHash, outIdx, err := GetTxHashAndOutIdx(tx, bs.netParams)
	if err != nil {
		return false, fmt.Errorf("failed to get tx hash and funding output index: %v", err)
	}
	// we make use of GetTxOut, which returns a non-nil UTXO if it's spendable
	// see https://developer.bitcoin.org/reference/rpc/gettxout.html for details
	// NOTE: we also consider mempool tx as per the last parameter
	txOut, err := bs.BTCClient.GetTxOut(txHash, outIdx, true)
	if err != nil {
		return false, fmt.Errorf(
			"failed to get the output of tx %s: %v",
			txHash.String(),
			err,
		)
	}
	if txOut == nil {
		log.Debugf(
			"tx %s output is already unspendable",
			txHash.String(),
		)
		return false, nil
	}
	// spendable
	return true, nil
}

func (bs *BTCSlasher) buildSlashingTxWithWitness(
	sk *btcec.PrivateKey,
	inputTx *bstypes.BabylonBTCTaprootTx,
	slashingTx *bstypes.BTCSlashingTx,
	delegatorSig *bbn.BIP340Signature,
	jurySig *bbn.BIP340Signature,
) (*wire.MsgTx, error) {
	inMsgTx, err := inputTx.ToMsgTx()
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to convert a Babylon BTC taproot tx to wire.MsgTx: %w", err)
	}
	inputScript := inputTx.Script
	valSig, err := slashingTx.Sign(inMsgTx, inputScript, sk, bs.netParams)
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to sign slashing tx for the BTC validator: %w", err)
	}

	// assemble witness for slashing tx
	slashingMsgTxWithWitness, err := slashingTx.ToMsgTxWithWitness(inputTx, valSig, delegatorSig, jurySig)
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to assemble witness for slashing tx: %v", err)
	}

	return slashingMsgTxWithWitness, nil
}

// GetTxHashAndOutIdx gets the tx hash and funding output's index of a Babylon BTC taproot tx,
// which can be either staking tx or unbonding tx
func GetTxHashAndOutIdx(tx *bstypes.BabylonBTCTaprootTx, net *chaincfg.Params) (*chainhash.Hash, uint32, error) {
	// get staking tx hash
	stakingMsgTx, err := tx.ToMsgTx()
	if err != nil {
		return nil, 0, fmt.Errorf(
			"failed to convert staking tx to MsgTx: %v",
			err,
		)
	}
	stakingMsgTxHash := stakingMsgTx.TxHash()

	// get staking tx output's index
	stakingOutIdx, err := btcstaking.GetIdxOutputCommitingToScript(stakingMsgTx, tx.Script, net)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"failed to get index of staking tx output: %v",
			err,
		)
	}

	return &stakingMsgTxHash, uint32(stakingOutIdx), nil
}
