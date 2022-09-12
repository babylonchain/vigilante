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
			log.Info("Polling sealed raw checkpoints...")
			sealedRawCkpts, err := s.babylonClient.QueryRawCheckpointList(checkpointingtypes.Sealed)
			if err != nil {
				log.Errorf("failed to query raw checkpoints: %v", err)
				continue
			}
			if len(sealedRawCkpts) == 0 {
				log.Info("Found no sealed raw checkpoints")
				continue
			}
			log.Infof("Found %d sealed raw checkpoints", len(sealedRawCkpts))
			log.Debugf("Accumulating raw checkpoints: %v", sealedRawCkpts)
			for _, ckpt := range sealedRawCkpts {
				s.rawCkptChan <- ckpt
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

// TODO: a new goroutine that consumes channel of raw ckpts, including
// - wrap a raw ckpt to two BTC txs
// - forward two BTC txs to BTC wallet/node
