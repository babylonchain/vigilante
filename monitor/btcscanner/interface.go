package btcscanner

import ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"

type Scanner interface {
	Start()
	GetNextCheckpoint() *ckpttypes.RawCheckpoint
}
