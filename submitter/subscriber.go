package submitter

const (
	SealedCheckpointEventKey string = "babylon.checkpointing.v1.EventCheckpointSealed.checkpoint"
)

func (s *Submitter) rawCheckpointSubscriber() {
	defer s.wg.Done()
	quit := s.quitChan()

	for {
		select {
		case bbnEvent := <-s.babylonClient.GetEvent():
			sealedCkptEvent, ok := bbnEvent.Events[SealedCheckpointEventKey]
			if ok {
				log.Infof("Received a Sealed Checkpoint event: %v", sealedCkptEvent)
				sealedCheckpoints, err := s.pollSealedRawCheckpoints()
				if err != nil {
					log.Errorf("Failed to poll Sealed checkpoints: %v", err)
					continue
				}
				if len(sealedCheckpoints) == 0 {
					log.Info("Found no Sealed checkpoints")
					continue
				}
				log.Infof("Found %v Sealed checkpoints", len(sealedCheckpoints))
				for _, ckpt := range sealedCheckpoints {
					s.rawCkptChan <- ckpt
				}
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}
