package types_test

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
)

// FuzzBtcCache fuzzes the BtcCache type
// 1. Generates BtcCache with random number of blocks.
// 2. Randomly add or remove blocks.
// 3. Find a random block.
// 4. Remove random blocks.
func FuzzBtcCache(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		// Create a new cache
		maxEntries := datagen.RandomInt(1000) + 2 // make sure we have at least 2 entries

		// 1/10 times generate invalid maxEntries
		if datagen.OneInN(10) {
			maxEntries = 0
		}

		cache, err := types.NewBTCCache(maxEntries)
		if err != nil {
			require.ErrorIs(t, err, types.ErrInvalidMaxEntries)
			return
		}

		// Generate a random number of blocks
		numBlocks := datagen.RandomIntOtherThan(0, int(maxEntries)) // make sure we have at least 1 entry

		// 1/10 times generate invalid number of blocks
		if datagen.OneInN(10) {
			numBlocks = maxEntries + 1
		}

		ibs := vdatagen.GetRandomIndexedBlocks(numBlocks)

		// Add all indexed blocks to the cache
		err = cache.Init(ibs)
		if err != nil {
			require.ErrorIs(t, err, types.ErrTooManyEntries)
			return
		}

		require.Equal(t, numBlocks, cache.Size())

		// Find a random block in the cache
		randIdx := datagen.RandomInt(int(numBlocks))
		randIb := ibs[randIdx]
		randIbHeight := uint64(randIb.Height)
		foundIb := cache.FindBlock(randIbHeight)
		require.NotNil(t, foundIb)
		require.Equal(t, foundIb, randIb)

		// Add random blocks to the cache
		addCount := datagen.RandomIntOtherThan(0, 1000)
		prevCacheHeight := cache.Tip().Height
		cacheBlocksBeforeAddition := cache.GetAllBlocks()
		blocksToAdd := vdatagen.GetRandomIndexedBlocksFromHeight(addCount, cache.Tip().Height, cache.Tip().BlockHash())
		for _, ib := range blocksToAdd {
			cache.Add(ib)
		}
		require.Equal(t, prevCacheHeight+int32(addCount), cache.Tip().Height)
		require.Equal(t, blocksToAdd[addCount-1], cache.Tip())

		// ensure block heights in cache are in increasing order
		var heights []int32
		for _, ib := range cache.GetAllBlocks() {
			heights = append(heights, ib.Height)
		}
		require.IsIncreasing(t, heights)

		// we need to compare block slices before and after addition, there are 3 cases to consider:
		// if addCount+numBlocks>=maxEntries then
		// 1. addCount >= maxEntries
		// 2. addCount < maxEntries
		// else
		// 3. addCount+numBlocks < maxEntries
		// case 2 and 3 are the same, so below is simplified version
		cacheBlocksAfterAddition := cache.GetAllBlocks()
		if addCount >= maxEntries {
			// if addCount >= maxEntries then all the blocks in cache are new blocks, compare
			// all cache blocks with slice of blocksToAdd
			require.Equal(t, blocksToAdd[addCount-maxEntries:], cacheBlocksAfterAddition)
		} else {
			// cache contains both old and all the new blocks, so we need to compare
			// blocksToAdd with slice of cacheBlocksAfterAddition and also compare
			// slice of cacheBlocksBeforeAddition with slice of cacheBlocksAfterAddition

			// compare new blocks
			newBlocksInCache := cacheBlocksAfterAddition[len(cacheBlocksAfterAddition)-int(addCount):]
			require.Equal(t, blocksToAdd, newBlocksInCache)

			// comparing old blocks
			oldBlocksInCache := cacheBlocksAfterAddition[:len(cacheBlocksAfterAddition)-int(addCount)]
			require.Equal(t, cacheBlocksBeforeAddition[len(cacheBlocksBeforeAddition)-(len(cacheBlocksAfterAddition)-int(addCount)):], oldBlocksInCache)
		}

		// Remove random number of blocks from the cache
		prevSize := cache.Size()
		deleteCount := datagen.RandomInt(int(prevSize))
		for i := 0; i < int(deleteCount); i++ {
			err = cache.RemoveLast()
			require.NoError(t, err)
		}
		require.Equal(t, prevSize-deleteCount, cache.Size())
		// check initial slice and expected output after deletion
	})
}
