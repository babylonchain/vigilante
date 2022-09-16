package types

import (
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"math/rand"
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

func Retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		attempts--
		if attempts > 0 {
			// Add some randomness to prevent thrashing
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			log.Infof("retry attempt %d, sleeping for %v sec", attempts, sleep)
			time.Sleep(sleep * time.Second)

			return Retry(attempts, 2*sleep, f)
		}

		return err

	}
	return nil
}
