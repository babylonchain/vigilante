package types_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
	"github.com/stretchr/testify/require"
)

func genRandomSegments(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion, match bool) (*types.CkptSegment, *types.CkptSegment) {
	rawBtcCkpt := &btctxformatter.RawBtcCheckpoint{
		Epoch:            rand.Uint64(),
		LastCommitHash:   datagen.GenRandomByteArray(btctxformatter.LastCommitHashLength),
		BitMap:           datagen.GenRandomByteArray(btctxformatter.BitMapLength),
		SubmitterAddress: datagen.GenRandomByteArray(btctxformatter.AddressLength),
		BlsSig:           datagen.GenRandomByteArray(btctxformatter.BlsSigLength),
	}
	firstHalf, secondHalf, err := btctxformatter.EncodeCheckpointData(
		tag,
		version,
		rawBtcCkpt,
	)
	if err != nil {
		panic(err)
	}
	// encode two halves to checkpoint segments
	bbnData1, err := btctxformatter.IsBabylonCheckpointData(tag, version, firstHalf)
	if err != nil {
		panic(err)
	}
	bbnData2, err := btctxformatter.IsBabylonCheckpointData(tag, version, secondHalf)
	if err != nil {
		panic(err)
	}

	// if we don't want a match, then mess up with one of BabylonData
	if !match {
		if datagen.OneInN(2) {
			lenData := uint64(len(bbnData1.Data))
			bbnData1.Data = datagen.GenRandomByteArray(lenData)
		} else {
			lenData := uint64(len(bbnData2.Data))
			bbnData2.Data = datagen.GenRandomByteArray(lenData)
		}
	}

	ckptSeg1 := &types.CkptSegment{
		BabylonData: bbnData1,
		TxIdx:       rand.Int(),
		AssocBlock:  nil,
	}
	ckptSeg2 := &types.CkptSegment{
		BabylonData: bbnData2,
		TxIdx:       rand.Int(),
		AssocBlock:  nil,
	}
	return ckptSeg1, ckptSeg2
}

func FuzzCheckpointCache(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		// get a random tag, either for mainnet or testnet
		var tag btctxformatter.BabylonTag
		if datagen.OneInN(2) {
			tag = btctxformatter.MainTag(uint8(rand.Uint32()))
		} else {
			tag = btctxformatter.TestTag(uint8(rand.Uint32()))
		}

		version := btctxformatter.CurrentVersion
		ckptCache := types.NewCheckpointCache(tag, version)

		numPairs := rand.Intn(200)
		numMatchedPairs := 0

		// add a random number of pairs of segments
		// where each pair may or may not match
		for i := 0; i < numPairs; i++ {
			var ckptSeg1, ckptSeg2 *types.CkptSegment
			lottery := rand.Float32()
			if lottery < 0.4 { // want a matched pair of segments
				ckptSeg1, ckptSeg2 = genRandomSegments(tag, version, true)
				numMatchedPairs++
			} else { // don't want a matched pair of segments
				ckptSeg1, ckptSeg2 = genRandomSegments(tag, version, false)
			}
			ckptCache.AddSegment(ckptSeg1)
			ckptCache.AddSegment(ckptSeg2)
			require.Equal(t, 2*(i+1), ckptCache.NumSegments())
		}

		ckptCache.Match()

		require.Equal(t, numMatchedPairs, ckptCache.NumCheckpoints())
		require.Equal(t, (numPairs-numMatchedPairs)*2, ckptCache.NumSegments())
	})
}
