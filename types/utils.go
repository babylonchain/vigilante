package types

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

func GetWrappedTxs(msg *wire.MsgBlock) []*btcutil.Tx {
	btcTxs := make([]*btcutil.Tx, len(msg.Transactions))

	for i := range msg.Transactions {
		newTx := btcutil.NewTx(msg.Transactions[i])
		newTx.Hash()
		newTx.WitnessHash()
		newTx.HasWitness()
		newTx.SetIndex(i)

		btcTxs = append(btcTxs, newTx)
	}

	return btcTxs
}
