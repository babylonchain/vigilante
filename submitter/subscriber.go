package submitter

const SealedCheckpointEventKey = "babylon.checkpointing.v1.EventCheckpointSealed.checkpoint"

func (s *Submitter) rawCheckpointSubscriber() {
	defer s.wg.Done()
	quit := s.quitChan()

	for {
		select {
		case bbnEvent := <-s.babylonClient.GetEvent():
			sealedCkptEvent, ok := bbnEvent.Events[SealedCheckpointEventKey]
			if ok {
				log.Infof("Received a Sealed Checkpoint event: %v", sealedCkptEvent[0])
				// TODO: parse the checkpoint and do submission
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}
