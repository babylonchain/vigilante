package types

import (
	"errors"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"math/rand"
	"time"
)

func isUnrecoverableErr(err error) bool {
	unrecoverableErrors := []error{
		btclctypes.ErrHeaderParentDoesNotExist.Wrap("parent for provided hash is not maintained"),
		btcctypes.ErrProvidedHeaderDoesNotHaveAncestor.Wrap("parent for provided hash is not maintained"),
		btcctypes.ErrUnknownHeader,
		btcctypes.ErrNoCheckpointsForPreviousEpoch,
		btcctypes.ErrInvalidCheckpointProof,
		// TODO Add more errors here
	}

	for _, e := range unrecoverableErrors {
		if errors.Is(err, e) {
			return true
		}
	}

	return false
}

func isExpectedErr(err error) bool {
	expectedErrors := []error{
		btclctypes.ErrDuplicateHeader.Wrap("header with provided hash already exists"),
		btcctypes.ErrDuplicatedSubmission,
		btcctypes.ErrUnknownHeader,
		// TODO Add more errors here
	}

	for _, e := range expectedErrors {
		if errors.Is(err, e) {
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
