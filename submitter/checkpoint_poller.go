package submitter

import (
	"time"

	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
)

func (s *Submitter) rawCheckpointPoller() {
	defer s.wg.Done()
	quit := s.quitChan()

	ticker := time.NewTicker(5 * time.Second) // TODO: parameterise polling frequency

	for {
		select {
		case <-ticker.C:
			log.Info("Polling accumulating raw checkpoints...")
			accumulatingRawCkpts, err := s.babylonClient.QueryRawCheckpointList(checkpointingtypes.Accumulating)
			if err != nil {
				log.Errorf("failed to query raw checkpoints: %v", err)
				continue
			}
			if len(accumulatingRawCkpts) == 0 {
				log.Info("Found no accumulating raw checkpoint")
				continue
			}
			log.Infof("Found %d accumulating raw checkpoints", len(accumulatingRawCkpts))
			log.Debugf("Accumulating raw checkpoints: %v", accumulatingRawCkpts)
			// TODO: enqueue new raw ckpts to a channel
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

// TODO: a new goroutine that consumes channel of raw ckpts, including
// - wrap a raw ckpt to two BTC txs
// - forward two BTC txs to BTC wallet/node
