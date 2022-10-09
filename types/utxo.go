package types

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type UTXO struct {
	TxID     *chainhash.Hash
	Vout     uint32
	ScriptPK []byte
	Amount   btcutil.Amount
	Addr     btcutil.Address
}
