package relayer

import (
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"

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

func calTxSize(tx *wire.MsgTx, utxo *types.UTXO, changeScript []byte) (uint64, error) {
	tx.AddTxOut(wire.NewTxOut(int64(utxo.Amount), changeScript))

	// when calculating tx size we can use a random private key
	privKey, err := secp.GeneratePrivateKey()
	if err != nil {
		return 0, err
	}

	// add signature/witness depending on the type of the previous address
	// if not segwit, add signature; otherwise, add witness
	segwit, err := isSegWit(utxo.Addr)
	if err != nil {
		panic(err)
	}

	tx, err = completeTxIn(tx, segwit, privKey, utxo)
	if err != nil {
		return 0, err
	}

	return uint64(tx.SerializeSizeStripped()), nil
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
