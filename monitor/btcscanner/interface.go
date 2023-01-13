package btcscanner

import (
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/types"
)

type Scanner interface {
	Start()
	GetNextCheckpoint() *ckpttypes.RawCheckpoint
	GetNextConfirmedBlock() *types.IndexedBlock
}
