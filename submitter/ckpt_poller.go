package submitter

import (
	"time"

	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
)

func (s *Submitter) rawCheckpointPoller() {
	defer s.wg.Done()
	quit := s.quitChan()

	ticker := time.NewTicker(time.Duration(s.Cfg.PollingFrequency) * time.Second)

	for {
		select {
		case <-ticker.C:
			log.Info("Polling sealed raw checkpoints...")
			sealedRawCkpts, err := s.babylonClient.QueryRawCheckpointList(checkpointingtypes.Sealed)
			log.Debugf("Next polling happens in %v seconds", s.Cfg.PollingFrequency)
			if err != nil {
				log.Errorf("failed to query raw checkpoints: %v", err)
				continue
			}
			if len(sealedRawCkpts) == 0 {
				log.Info("Found no sealed raw checkpoints")
				continue
			}
			log.Infof("Found %d sealed raw checkpoints", len(sealedRawCkpts))
			for _, ckpt := range sealedRawCkpts {
				s.rawCkptChan <- ckpt
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}
