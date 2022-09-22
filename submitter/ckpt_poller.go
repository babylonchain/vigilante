package submitter

import (
	"time"

	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
)

func (s *Submitter) rawCheckpointPoller() {
	defer s.wg.Done()
	quit := s.quitChan()

	ticker := time.NewTicker(time.Duration(s.Cfg.PollingIntervalSeconds) * time.Second)

	for {
		select {
		case <-ticker.C:
			log.Info("Polling sealed raw checkpoints...")
			sealedRawCkpts, err := s.pollSealedRawCheckpoints()
			log.Debugf("Next polling happens in %v seconds", s.Cfg.PollingIntervalSeconds)
			if err != nil {
				log.Errorf("Failed to query raw checkpoints: %v", err)
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

func (s *Submitter) pollSealedRawCheckpoints() ([]*checkpointingtypes.RawCheckpointWithMeta, error) {
	sealedRawCkpts, err := s.babylonClient.QueryRawCheckpointList(checkpointingtypes.Sealed)
	if err != nil {
		return nil, err
	}

	return sealedRawCkpts, err
}
