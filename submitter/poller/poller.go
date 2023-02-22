package poller

import (
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/rpc-client/query"

	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/querier"
)

type Poller struct {
	querier     *querier.BabylonQuerier
	bufferSize  uint
	rawCkptChan chan *checkpointingtypes.RawCheckpointWithMeta
}

func New(client query.BabylonQueryClient, bufferSize uint) *Poller {
	return &Poller{
		rawCkptChan: make(chan *checkpointingtypes.RawCheckpointWithMeta, bufferSize),
		bufferSize:  bufferSize,
		querier:     querier.New(client),
	}
}

// PollSealedCheckpoints polls raw checkpoints with the status of Sealed
// and pushes the oldest one into the channel
func (pl *Poller) PollSealedCheckpoints() error {
	sealedCheckpoints, err := pl.querier.RawCheckpointList(checkpointingtypes.Sealed)
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
