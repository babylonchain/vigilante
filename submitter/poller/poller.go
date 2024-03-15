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

	rawCkptWithMeta, err := newRawCheckpointWithMetaFromResponse(oldestCkpt)
	if err != nil {
		return err
	}
	pl.rawCkptChan <- rawCkptWithMeta

	return nil
}

func (pl *Poller) GetSealedCheckpointChan() <-chan *checkpointingtypes.RawCheckpointWithMeta {
	return pl.rawCkptChan
}

func newRawCheckpointWithMetaFromResponse(resp *checkpointingtypes.RawCheckpointWithMetaResponse) (*checkpointingtypes.RawCheckpointWithMeta, error) {
	rawCkpt, err := resp.Ckpt.ToRawCheckpoint()
	if err != nil {
		return nil, err
	}
	rawCkptWithMeta := &checkpointingtypes.RawCheckpointWithMeta{
		Ckpt:      rawCkpt,
		Status:    resp.Status,
		BlsAggrPk: resp.BlsAggrPk,
		PowerSum:  resp.PowerSum,
		Lifecycle: []*checkpointingtypes.CheckpointStateUpdate{},
	}
	for i := range resp.Lifecycle {
		lc := &checkpointingtypes.CheckpointStateUpdate{
			State:       resp.Lifecycle[i].State,
			BlockHeight: resp.Lifecycle[i].BlockHeight,
			BlockTime:   resp.Lifecycle[i].BlockTime,
		}
		rawCkptWithMeta.Lifecycle = append(rawCkptWithMeta.Lifecycle, lc)
	}

	return rawCkptWithMeta, nil
}
