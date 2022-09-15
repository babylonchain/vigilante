package types

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
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

func Retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		attempts--
		if attempts > 0 {
			log.Infof("retry attempt %d, sleeping for %v sec", attempts, sleep)
			time.Sleep(sleep)

			return Retry(attempts, sleep, f)
		}

		return err

	}
	return nil
}
