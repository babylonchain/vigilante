package types

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/google/martian/log"
	"time"
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

func Retry(attempts int, f func() error) error {
	if err := f(); err != nil {
		attempts--
		if attempts > 0 {
			log.Infof("retry attempt %d, sleeping for 1 sec", attempts)
			time.Sleep(1)

			return Retry(attempts, f)
		}

		return err

	}
	return nil
}
