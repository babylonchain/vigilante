package types

import ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"

type CheckpointBTC struct {
	*ckpttypes.RawCheckpoint
	// the BTC height at which the first checkpoint is included
	BtcHeight uint64
}
