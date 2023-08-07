package poller

import (
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
)

type Poller struct {
	querier     BabylonQueryClient
	bufferSize  uint
	rawCkptChan chan *checkpointingtypes.RawCheckpointWithMeta
}

func New(client BabylonQueryClient, bufferSize uint) *Poller {
	return &Poller{
		rawCkptChan: make(chan *checkpointingtypes.RawCheckpointWithMeta, bufferSize),
		bufferSize:  bufferSize,
		querier:     client,
	}
}

// PollSealedCheckpoints polls raw checkpoints with the status of Sealed
// and pushes the oldest one into the channel
func (pl *Poller) PollSealedCheckpoints() error {
	res, err := pl.querier.RawCheckpointList(checkpointingtypes.Sealed, nil)
	if err != nil {
		return err
	}
	sealedCheckpoints := res.RawCheckpoints

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

	return nil
}

func (pl *Poller) GetSealedCheckpointChan() <-chan *checkpointingtypes.RawCheckpointWithMeta {
	return pl.rawCkptChan
}
