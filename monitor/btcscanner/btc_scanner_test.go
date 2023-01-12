package btcscanner_test

import (
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/vigilante/monitor/btcscanner"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/babylonchain/vigilante/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func FuzzBootStrap(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)
		k := datagen.RandomIntOtherThan(0, 10)
		// Generate a random number of blocks
		numBlocks := datagen.RandomIntOtherThan(0, 1000) + k // make sure we have at least k+1 entry
		chainIndexedBlocks := vdatagen.GetRandomIndexedBlocks(numBlocks)
		baseHeight := uint64(chainIndexedBlocks[0].Height)
		ctl := gomock.NewController(t)
		mockBtcClient := mocks.NewMockBTCClient(ctl)
		canonicalChain := chainIndexedBlocks[:numBlocks-k-1]
		tailChain := chainIndexedBlocks[numBlocks-k:]
		mockBtcClient.EXPECT().MustSubscribeBlocks().Return().AnyTimes()
		mockBtcClient.EXPECT().FindTailBlocks(k).Return(tailChain, nil).AnyTimes()
		mockBtcClient.EXPECT().GetBlockByHash(&tailChain[0].Header.PrevBlock).Return(canonicalChain[len(canonicalChain)-1], nil, nil).AnyTimes()
		mockBtcClient.EXPECT().GetChainBlocks(baseHeight, canonicalChain[len(canonicalChain)-1]).Return(canonicalChain, nil).AnyTimes()

		cache, err := types.NewBTCCache(10)
		require.NoError(t, err)
		btcScanner := &btcscanner.BtcScanner{
			BtcClient:           mockBtcClient,
			BaseHeight:          baseHeight,
			K:                   k,
			CanonicalBlocksChan: make(chan *types.IndexedBlock, 0),
			TailBlocks:          cache,
		}
		go func() {
			for i := 0; i < len(canonicalChain); i++ {
				b := btcScanner.GetNextCanonicalBlock()
				require.Equal(t, canonicalChain[i].BlockHash(), b.BlockHash())
			}
		}()
		btcScanner.Bootstrap()
		require.Equal(t, uint64(len(tailChain)), btcScanner.TailBlocks.Size())
		require.Equal(t, tailChain[len(tailChain)-1].BlockHash(), btcScanner.TailBlocks.Tip().BlockHash())
	})
}
