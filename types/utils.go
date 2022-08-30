package types

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

func getWrappedTxs(msg *wire.MsgBlock) []*btcutil.Tx {
	btcTx := make([]*btcutil.Tx, len(msg.Transactions))

	for i := range msg.Transactions {
		newTx := btcutil.NewTx(msg.Transactions[i])
		newTx.Hash()
		newTx.WitnessHash()
		newTx.HasWitness()
		newTx.SetIndex(i)
	}

	return btcTx
}
