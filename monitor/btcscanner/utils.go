package btcscanner

import (
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

func GetWrappedTxs(msg *wire.MsgBlock) []*btcutil.Tx {
	btcTxs := []*btcutil.Tx{}

	for i := range msg.Transactions {
		newTx := btcutil.NewTx(msg.Transactions[i])
		newTx.SetIndex(i)

		btcTxs = append(btcTxs, newTx)
	}

	return btcTxs
}
