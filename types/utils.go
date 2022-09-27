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

func isUnrecoverableErr(err error) bool {
	unrecoverableErrors := []string{
		btclctypes.ErrHeaderParentDoesNotExist.Error(),
		btcctypes.ErrProvidedHeaderDoesNotHaveAncestor.Error(),
		btcctypes.ErrUnknownHeader.Error(),
		btcctypes.ErrNoCheckpointsForPreviousEpoch.Error(),
		btcctypes.ErrInvalidCheckpointProof.Error(),
		// TODO Add more errors here
	}

	for _, e := range unrecoverableErrors {
		if strings.Contains(err.Error(), e) {
			return true
		}
	}

	return false
}

func isExpectedErr(err error) bool {
	expectedErrors := []string{
		btclctypes.ErrDuplicateHeader.Error(),
		btcctypes.ErrDuplicatedSubmission.Error(),
		btcctypes.ErrUnknownHeader.Error(),
		// TODO Add more errors here
	}

	for _, e := range expectedErrors {
		if strings.Contains(err.Error(), e) {
			return true
		}
	}

	return false
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

func Retry(sleep time.Duration, maxSleepTime time.Duration, retryableFunc func() error) error {
	if err := retryableFunc(); err != nil {
		if isUnrecoverableErr(err) {
			log.Warnf("Skip retry, error unrecoverable %v", err)
			return err
		}

		if isExpectedErr(err) {
			log.Warnf("Skip retry, error expected %v", err)
			return nil
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

		return Retry(2*sleep, maxSleepTime, retryableFunc)
	}
	return nil
}
