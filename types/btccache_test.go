package types_test

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
)

func FuzzBtcCache(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		// Create a new cache
		maxEntries := datagen.RandomInt(100) + 2 // ensure maxEntries > 1
		cache, err := types.NewBTCCache(maxEntries)
		require.NoError(t, err)

		// Generate a random number of blocks
		numBlocks := datagen.RandomIntOtherThan(0, int(maxEntries)) // ensure numBlocks > 0
		ibs := vdatagen.GetRandomIndexedBlocks(numBlocks)

		// Add all indexed blocks to the cache
		err = cache.Init(ibs)
		require.NoError(t, err)
		require.Equal(t, numBlocks, cache.Size())

		// Find a random block in the cache
		randIdx := datagen.RandomInt(int(numBlocks))
		randIb := ibs[randIdx]
		randIbHeight := uint64(randIb.Height)
		foundIb := cache.FindBlock(randIbHeight)
		require.NotNil(t, foundIb)

		// Get prev hash
		tip := cache.Tip()
		require.NotNil(t, tip)
		prevHash := tip.Header.BlockHash()

		// Add a new block to the cache
		block, _ := vdatagen.GenRandomBlock(1, &prevHash)
		newIb := types.NewIndexedBlockFromMsgBlock(rand.Int31(), block)
		cache.Add(newIb)
		require.Equal(t, numBlocks+1, cache.Size())

		// Remove random number of blocks from the cache
		prevSize := cache.Size()
		deleteCount := datagen.RandomInt(int(prevSize))
		for i := 0; i < int(deleteCount); i++ {
			err = cache.RemoveLast()
			require.NoError(t, err)
		}
		require.Equal(t, prevSize-deleteCount, cache.Size())
	})
}
