package poller

import (
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/log"
)

type Poller struct {
	bbnclient.BabylonClient
	bufferSize  uint
	rawCkptChan chan *checkpointingtypes.RawCheckpointWithMeta
}

func New(client bbnclient.BabylonClient, bufferSize uint) *Poller {
	return &Poller{
		rawCkptChan:   make(chan *checkpointingtypes.RawCheckpointWithMeta, bufferSize),
		bufferSize:    bufferSize,
		BabylonClient: client,
	}
}

// PollSealedCheckpoints polls raw checkpoints with the status of Sealed
// and pushes the oldest one into the channel
func (pl *Poller) PollSealedCheckpoints() error {
	sealedCheckpoints, err := pl.QueryRawCheckpointList(checkpointingtypes.Sealed)
	if err != nil {
		return err
	}

	if len(sealedCheckpoints) == 0 {
		return nil
	}

	// the QueryRawCheckpointList should return checkpoints in the ascending order of the epoch number
	// this is to make sure the oldest one is chosen
	oldestCkpt := sealedCheckpoints[0]
	for _, ckpt := range sealedCheckpoints {
		if oldestCkpt.Ckpt.EpochNum > ckpt.Ckpt.EpochNum {
			oldestCkpt = ckpt
		}
	}

	pl.rawCkptChan <- oldestCkpt
	log.Logger.Infof("a sealed checkpoint for epoch %v is polled", oldestCkpt.Ckpt.EpochNum)

	return nil
}

func (pl *Poller) GetSealedCheckpointChan() <-chan *checkpointingtypes.RawCheckpointWithMeta {
	return pl.rawCkptChan
}

func (pl *Poller) Stop() {
	if pl.BabylonClient != nil {
		pl.BabylonClient.Stop()
		pl.BabylonClient = nil
	}
}
