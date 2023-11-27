package btcslasher

import (
	"fmt"
	"github.com/babylonchain/babylon/btcstaking"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
)

// isTaprootOutputSpendable checks if the taproot output of a given tx is still spendable on Bitcoin
// This function can be used to check the output of a staking tx or an undelegation tx
func (bs *BTCSlasher) isTaprootOutputSpendable(txBytes []byte, outIdx uint32) (bool, error) {
	stakingMsgTx, err := bstypes.ParseBtcTx(txBytes)
	if err != nil {
		return false, fmt.Errorf(
			"failed to convert staking tx to MsgTx: %v",
			err,
		)
	}
	stakingMsgTxHash := stakingMsgTx.TxHash()

	// we make use of GetTxOut, which returns a non-nil UTXO if it's spendable
	// see https://developer.bitcoin.org/reference/rpc/gettxout.html for details
	// NOTE: we also consider mempool tx as per the last parameter
	txOut, err := bs.BTCClient.GetTxOut(&stakingMsgTxHash, outIdx, true)
	if err != nil {
		return false, fmt.Errorf(
			"failed to get the output of tx %s: %v",
			stakingMsgTxHash.String(),
			err,
		)
	}
	if txOut == nil {
		log.Debugf(
			"tx %s output is already unspendable",
			stakingMsgTxHash.String(),
		)
		return false, nil
	}
	// spendable
	return true, nil
}

func (bs *BTCSlasher) buildSlashingTxWithWitness(
	sk *btcec.PrivateKey,
	inputTxBytes []byte,
	outputIdx uint32,
	slashingTx *bstypes.BTCSlashingTx,
	delegatorSig *bbn.BIP340Signature,
	covenantSig *bbn.BIP340Signature,
	slashingPathSpendInfo *btcstaking.SpendInfo,
) (*wire.MsgTx, error) {
	inMsgTx, err := bstypes.ParseBtcTx(inputTxBytes)
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to convert a Babylon BTC taproot tx to wire.MsgTx: %w", err)
	}
	valSig, err := slashingTx.Sign(inMsgTx, outputIdx, slashingPathSpendInfo.RevealedLeaf.Script, sk, bs.netParams)
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to sign slashing tx for the BTC validator: %w", err)
	}

	stakerSigBytes := delegatorSig.MustMarshal()
	validatorSigBytes := valSig.MustMarshal()
	covSigBytes := covenantSig.MustMarshal()

	// TODO: use committee
	witness, err := btcstaking.CreateBabylonWitness(
		[][]byte{
			covSigBytes,
			validatorSigBytes,
			stakerSigBytes,
		},
		slashingPathSpendInfo,
	)
	if err != nil {
		return nil, err
	}
	slashingMsgTxWithWitness, err := slashingTx.ToMsgTx()
	if err != nil {
		return nil, err
	}
	slashingMsgTxWithWitness.TxIn[0].Witness = witness

	return slashingMsgTxWithWitness, nil
}
