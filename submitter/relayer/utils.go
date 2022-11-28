package relayer

import (
	"errors"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const dummyChangeValue int64 = 2500000000

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

func calTxSize(tx *wire.MsgTx, utxo *types.UTXO, changeScript []byte, isSegWit bool, privkey *btcec.PrivateKey) (uint64, error) {
	tx.AddTxOut(wire.NewTxOut(dummyChangeValue, changeScript))
	if !isSegWit {
		sig, err := txscript.SignatureScript(
			tx,
			0,
			utxo.ScriptPK,
			txscript.SigHashAll,
			privkey,
			true)
		if err != nil {
			return 0, err
		}
		tx.TxIn[0].SignatureScript = sig
	} else {
		sighashes := txscript.NewTxSigHashes(tx)
		wit, err := txscript.WitnessSignature(
			tx,
			sighashes,
			0,
			int64(utxo.Amount),
			utxo.ScriptPK,
			txscript.SigHashAll,
			privkey,
			true,
		)
		if err != nil {
			return 0, err
		}
		tx.TxIn[0].Witness = wit
	}
	txSize := tx.SerializeSize()

	// remove dummy tx out
	tx.TxOut = tx.TxOut[:len(tx.TxOut)-1]

	return uint64(txSize), nil
}
