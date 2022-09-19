package submitter

import (
	"fmt"
	"github.com/gogo/protobuf/proto"

	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
)

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
				ckpt := new(checkpointingtypes.RawCheckpointWithMeta)
				err := proto.Unmarshal([]byte(sealedCkptEvent[0]), ckpt)
				if err != nil {
					log.Errorf("Failed to unmarshal a sealed checkpoint %v", err)
				}
				fmt.Printf("Raw checkpoint: %v", ckpt.Ckpt)
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}
