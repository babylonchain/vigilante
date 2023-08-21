package relayer

import (
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet/txsizes"

	"github.com/babylonchain/vigilante/types"
)

func isSegWit(addr btcutil.Address) (bool, error) {
	switch addr.(type) {
	case *btcutil.AddressPubKeyHash, *btcutil.AddressScriptHash, *btcutil.AddressPubKey:
		return false, nil
	case *btcutil.AddressWitnessPubKeyHash, *btcutil.AddressWitnessScriptHash:
		return true, nil
	default:
		return false, errors.New("non-supported address type")
	}
}

func calculateTxSize(tx *wire.MsgTx, utxo *types.UTXO, changeScript []byte) int {
	tx.AddTxOut(wire.NewTxOut(int64(utxo.Amount), changeScript))

	// We count the types of inputs, which we'll use to estimate
	// the vsize of the transaction.
	var nested, p2wpkh, p2tr, p2pkh int
	switch {
	// If this is a p2sh output, we assume this is a
	// nested P2WKH.
	case txscript.IsPayToScriptHash(utxo.ScriptPK):
		nested++
	case txscript.IsPayToWitnessPubKeyHash(utxo.ScriptPK):
		p2wpkh++
	case txscript.IsPayToTaproot(utxo.ScriptPK):
		p2tr++
	default:
		p2pkh++
	}

	return txsizes.EstimateVirtualSize(p2pkh, p2tr, p2wpkh, nested, tx.TxOut, len(changeScript))
}

func completeTxIn(tx *wire.MsgTx, isSegWit bool, privKey *btcec.PrivateKey, utxo *types.UTXO) (*wire.MsgTx, error) {
	if !isSegWit {
		sig, err := txscript.SignatureScript(
			tx,
			0,
			utxo.ScriptPK,
			txscript.SigHashAll,
			privKey,
			true,
		)
		if err != nil {
			return nil, err
		}
		tx.TxIn[0].SignatureScript = sig
	} else {
		sighashes := txscript.NewTxSigHashes(
			tx,
			// Use the CannedPrevOutputFetcher which is only able to return information about a single UTXO
			// See https://github.com/btcsuite/btcd/commit/e781b66e2fb9a354a14bfa7fbdd44038450cc13f
			// for details on the output fetchers
			txscript.NewCannedPrevOutputFetcher(utxo.ScriptPK, int64(utxo.Amount)))
		wit, err := txscript.WitnessSignature(
			tx,
			sighashes,
			0,
			int64(utxo.Amount),
			utxo.ScriptPK,
			txscript.SigHashAll,
			privKey,
			true,
		)
		if err != nil {
			return nil, err
		}
		tx.TxIn[0].Witness = wit
	}

	return tx, nil
}
