package btcscanner

import (
	"github.com/babylonchain/vigilante/types"
)

type Scanner interface {
	Start()
	GetNextCheckpoint() *types.CheckpointBTC
	GetNextConfirmedBlock() *types.IndexedBlock
}
