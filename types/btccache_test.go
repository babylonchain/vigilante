package types_test

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
)

func getRandomIndexedBlocks(numBlocks uint64) []*types.IndexedBlock {
	blocks, _, _ := vdatagen.GenRandomBlockchainWithBabylonTx(numBlocks, 0, 0)
	var ibs []*types.IndexedBlock
	for _, block := range blocks {
		ibs = append(ibs, types.NewIndexedBlockFromMsgBlock(rand.Int31(), block))
	}

	return ibs
}

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
		ibs := getRandomIndexedBlocks(numBlocks)

		// Add all indexed blocks to the cache
		err = cache.Init(ibs)
		require.NoError(t, err)
		require.Equal(t, numBlocks, cache.Size())

		// Get prev hash
		tip := cache.Tip()
		require.NotNil(t, tip)
		prevHash := tip.Header.BlockHash()

		// Add a new block to the cache
		block, _ := vdatagen.GenRandomBlock(1, &prevHash)
		newIb := types.NewIndexedBlockFromMsgBlock(rand.Int31(), block)
		cache.Add(newIb)

		// Remove the last block from the cache
		err = cache.RemoveLast()
		require.NoError(t, err)
	})
}
