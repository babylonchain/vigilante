package types

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"time"
)

type CheckpointInfo struct {
	ts       *time.Time
	btcTxId1 *chainhash.Hash
	btcTxId2 *chainhash.Hash
}

type SentCheckpoints struct {
	resendIntervalSeconds uint
	checkpoints           map[uint64]*CheckpointInfo
}

func NewSentCheckpoints(interval uint) SentCheckpoints {
	return SentCheckpoints{
		resendIntervalSeconds: interval,
		checkpoints:           make(map[uint64]*CheckpointInfo, 0),
	}
}

// ShouldSend returns true if
// 1. no checkpoint was sent for the epoch
// 2. the last sent time is outdated by resendIntervalSeconds
func (sc *SentCheckpoints) ShouldSend(epoch uint64) bool {
	ckptInfo, ok := sc.checkpoints[epoch]
	// 1. no checkpoint was sent for the epoch
	if !ok {
		log.Debugf("The checkpoint for epoch %v has never been sent, should send", epoch)
		return true
	}
	// 2. should resend if some interval has passed since the last sent
	durSeconds := uint(time.Since(*ckptInfo.ts).Seconds())
	if durSeconds >= sc.resendIntervalSeconds {
		log.Debugf("The checkpoint for epoch %v was sent more than %v seconds ago, should resend", epoch, sc.resendIntervalSeconds)
		return true
	}

	log.Debugf("The checkpoint for epoch %v was sent at %v, should not resend", epoch, ckptInfo.ts)

	return false
}

// Add adds a newly sent checkpoint info
func (sc *SentCheckpoints) Add(epoch uint64, txid1 *chainhash.Hash, txid2 *chainhash.Hash) {
	ts := time.Now()
	ckptInfo := &CheckpointInfo{
		ts:       &ts,
		btcTxId1: txid1,
		btcTxId2: txid2,
	}
	sc.checkpoints[epoch] = ckptInfo
}
