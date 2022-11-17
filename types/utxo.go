package types

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
)

type UTXO struct {
	TxID     *chainhash.Hash
	Vout     uint32
	ScriptPK []byte
	Amount   btcutil.Amount
	Addr     btcutil.Address
}
