package submitter

import (
	"fmt"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/gogo/protobuf/proto"
)

const (
	SealedCheckpointEventKey string = "babylon.checkpointing.v1.EventCheckpointSealed.checkpoint"
	EpochNumKey              string = "epoch_num"
)

func (s *Submitter) rawCheckpointSubscriber() {
	defer s.wg.Done()
	quit := s.quitChan()

	for {
		select {
		case bbnEvent := <-s.babylonClient.GetEvent():
			sealedCkptEvent, ok := bbnEvent.Events[SealedCheckpointEventKey]
			if ok {
				log.Infof("Received a Sealed Checkpoint event: %v, type: %T", sealedCkptEvent[0], sealedCkptEvent[0])
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
