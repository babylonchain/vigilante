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
				panic(err) // TODO: better error handling?
			}
			log.Infof("Found %d sealed raw checkpoints: %v", len(sealedRawCkpts), sealedRawCkpts)
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
