package datagen

import (
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
)

func GenerateRandomCheckpointRecord() *types.CheckpointRecord {
	rawCheckpoint := datagen.GenRandomRawCheckpoint()
	btcHeight := datagen.RandomIntOtherThan(0, 1000)

	return &types.CheckpointRecord{
		RawCheckpoint:      rawCheckpoint,
		FirstSeenBtcHeight: btcHeight,
	}
}
