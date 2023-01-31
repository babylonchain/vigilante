package types

import (
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
)

type CheckpointRecord struct {
	RawCheckpoint      *ckpttypes.RawCheckpoint
	FirstSeenBtcHeight uint64
}

type CheckpointsBuffer struct {
	ckpts []*CheckpointRecord
}

func NewCheckpointRecord(ckpt *ckpttypes.RawCheckpoint, height uint64) *CheckpointRecord {
	return &CheckpointRecord{RawCheckpoint: ckpt, FirstSeenBtcHeight: height}
}

// ID returns the hash of the raw checkpoint
func (cr *CheckpointRecord) ID() string {
	return cr.RawCheckpoint.Hash().String()
}

func (cr *CheckpointRecord) EpochNum() uint64 {
	return cr.RawCheckpoint.EpochNum
}

func (cb *CheckpointsBuffer) Add(ckptRecord *CheckpointRecord) {
	cb.ckpts = append(cb.ckpts, ckptRecord)
}

func (cb *CheckpointsBuffer) Front() *CheckpointRecord {
	if len(cb.ckpts) == 0 {
		return nil
	}
	return cb.ckpts[0]
}
