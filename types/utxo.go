package types

import (
	"encoding/hex"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type UTXO struct {
	TxID     *chainhash.Hash
	Vout     uint32
	ScriptPK []byte
	Amount   btcutil.Amount
	Addr     btcutil.Address
}

func NewUTXO(r *btcjson.ListUnspentResult, net *chaincfg.Params) (*UTXO, error) {
	prevPKScript, err := hex.DecodeString(r.ScriptPubKey)
	if err != nil {
		return nil, err
	}
	txID, err := chainhash.NewHashFromStr(r.TxID)
	if err != nil {
		return nil, err
	}
	prevAddr, err := btcutil.DecodeAddress(r.Address, net)
	if err != nil {
		return nil, err
	}
	amount, err := btcutil.NewAmount(r.Amount)
	if err != nil {
		return nil, err
	}

	utxo := &UTXO{
		TxID:     txID,
		Vout:     r.Vout,
		ScriptPK: prevPKScript,
		Amount:   amount,
		Addr:     prevAddr,
	}
	return utxo, nil
}

func (u *UTXO) GetOutPoint() *wire.OutPoint {
	return wire.NewOutPoint(u.TxID, u.Vout)
}
