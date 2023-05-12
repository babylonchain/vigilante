package datagen

import (
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
	"math/rand"
)

func GenerateRandomCheckpointRecord(r *rand.Rand) *types.CheckpointRecord {
	rawCheckpoint := datagen.GenRandomRawCheckpoint(r)
	btcHeight := datagen.RandomIntOtherThan(r, 0, 1000)

	return &types.CheckpointRecord{
		RawCheckpoint:      rawCheckpoint,
		FirstSeenBtcHeight: btcHeight,
	}
}
