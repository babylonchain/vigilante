package types

import (
	"time"

	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"math/rand"
	"strings"
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

func Retry(sleep time.Duration, timeout time.Duration, f func() error) error {
	if err := f(); err != nil {
		if strings.Contains(err.Error(), btclctypes.ErrDuplicateHeader.Error()) {
			log.Warnf("Ignoring the error of duplicate headers")
			return nil
		}

		if strings.Contains(err.Error(), btclctypes.ErrHeaderParentDoesNotExist.Error()) {
			return err
		}

		for {
			// Add some randomness to prevent thrashing
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			if sleep > timeout {
				log.Info("retry timed out")
				return err
			}

			log.Infof("sleeping for %v sec", sleep)
			time.Sleep(sleep)

			return Retry(2*sleep, timeout, f)
		}
	}
	return nil
}
