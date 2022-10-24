package types_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
	"github.com/stretchr/testify/require"
)

func randNBytes(n int) []byte {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return bytes
}

func getRandomRawCheckpoint() *btctxformatter.RawBtcCheckpoint {
	return &btctxformatter.RawBtcCheckpoint{
		Epoch:            rand.Uint64(),
		LastCommitHash:   randNBytes(btctxformatter.LastCommitHashLength),
		BitMap:           randNBytes(btctxformatter.BitMapLength),
		SubmitterAddress: randNBytes(btctxformatter.AddressLength),
		BlsSig:           randNBytes(btctxformatter.BlsSigLength),
	}
}

func FuzzCkptSegmentPool(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		tag := btctxformatter.MainTag(48)
		version := btctxformatter.CurrentVersion
		pool := types.NewCkptSegmentPool(tag, version)

		// fake a raw checkpoint
		rawBTCCkpt := getRandomRawCheckpoint()
		// encode raw checkpoint to two halves
		firstHalf, secondHalf, err := btctxformatter.EncodeCheckpointData(
			tag,
			version,
			rawBTCCkpt,
		)
		require.NoError(t, err)
		require.NotNil(t, firstHalf)
		require.NotNil(t, secondHalf)

		// encode two halves to checkpoint segments
		bbnData1, err := btctxformatter.IsBabylonCheckpointData(tag, version, firstHalf)
		require.NoError(t, err)
		bbnData2, err := btctxformatter.IsBabylonCheckpointData(tag, version, secondHalf)
		require.NoError(t, err)

		ckptSeg1 := types.CkptSegment{
			BabylonData: bbnData1,
			TxIdx:       1,
			AssocBlock:  nil,
		}
		ckptSeg2 := types.CkptSegment{
			BabylonData: bbnData2,
			TxIdx:       2,
			AssocBlock:  nil,
		}

		// add two segments to the pool
		pool.Add(&ckptSeg1)
		pool.Add(&ckptSeg2)

		// find matched pairs of segments in the pool
		ckpts := pool.Match()

		// there should be exactly 1 checkpoint
		require.Len(t, ckpts, 1)
		require.Zero(t, pool.Size())
	})
}
