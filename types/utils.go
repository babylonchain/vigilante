package types

import (
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"math/rand"
	"strings"
	"time"
)

func IsRetryRequired(err error) bool {
	panicErrors := []string{
		btclctypes.ErrHeaderParentDoesNotExist.Error(),
		btcctypes.ErrProvidedHeaderDoesNotHaveAncestor.Error(),
		btcctypes.ErrUnknownHeader.Error(),
		btcctypes.ErrNoCheckpointsForPreviousEpoch.Error(),
		btcctypes.ErrInvalidCheckpointProof.Error(),
	}

	for _, e := range panicErrors {
		if strings.Contains(err.Error(), e) {
			return false
		}
	}

	return true
}

func GetRetryAcceptedErrors() []string {
	acceptedErrors := []string{
		btclctypes.ErrDuplicateHeader.Error(),
		btcctypes.ErrDuplicatedSubmission.Error(),
		btcctypes.ErrUnknownHeader.Error(),
	}

	return acceptedErrors
}

func GetWrappedTxs(msg *wire.MsgBlock) []*btcutil.Tx {
	btcTxs := []*btcutil.Tx{}

	for i := range msg.Transactions {
		newTx := btcutil.NewTx(msg.Transactions[i])
		newTx.SetIndex(i)

		btcTxs = append(btcTxs, newTx)
	}

	return btcTxs
}

func Retry(sleep time.Duration, maxSleepTime time.Duration, fn func() error, getAcceptedErrors func() []string) error {
	if err := fn(); err != nil {
		acceptedErrors := getAcceptedErrors()
		for _, e := range acceptedErrors {
			if strings.Contains(err.Error(), e) {
				log.Warnf("Error accepted, skip retry %v", err)
				return nil
			}
		}

		// Add some randomness to prevent thrashing
		jitter := time.Duration(rand.Int63n(int64(sleep)))
		sleep = sleep + jitter/2

		if sleep > maxSleepTime {
			log.Info("retry timed out")
			return err
		}

		log.Warnf("sleeping for %v sec: %v", sleep, err)
		time.Sleep(sleep)

		return Retry(2*sleep, maxSleepTime, fn, getAcceptedErrors)
	}
	return nil
}
