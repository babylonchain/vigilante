package types

import (
	"crypto/sha256"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO should be defined in Babylon
type CheckpointId [sha256.Size]byte

type CheckpointRecord struct {
	RawCheckpoint      *ckpttypes.RawCheckpoint
	FirstSeenBtcHeight uint64
}

func NewCheckpointRecord(ckpt *ckpttypes.RawCheckpoint, height uint64) *CheckpointRecord {
	return &CheckpointRecord{RawCheckpoint: ckpt, FirstSeenBtcHeight: height}
}

// ID returns the hash of the raw RawCheckpoint
// TODO: should be implemented as a method of RawCheckpoint
func (cr *CheckpointRecord) ID() CheckpointId {
	ckptBytes := make([]byte, 0)
	ckptBytes = append(ckptBytes, sdk.Uint64ToBigEndian(cr.RawCheckpoint.EpochNum)...)
	ckptBytes = append(ckptBytes, cr.RawCheckpoint.LastCommitHash.MustMarshal()...)
	ckptBytes = append(ckptBytes, cr.RawCheckpoint.Bitmap...)
	ckptBytes = append(ckptBytes, cr.RawCheckpoint.BlsMultiSig.MustMarshal()...)

	return sha256.Sum256(ckptBytes)
}

func (cr *CheckpointRecord) EpochNum() uint64 {
	return cr.RawCheckpoint.EpochNum
}
