package btcscanner_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/monitor/btcscanner"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/babylonchain/vigilante/types"
)

func FuzzBootStrap(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		k := datagen.RandomIntOtherThan(r, 0, 10)
		// Generate a random number of blocks
		numBlocks := datagen.RandomIntOtherThan(r, 0, 100) + k // make sure we have at least k+1 entry
		chainIndexedBlocks := vdatagen.GetRandomIndexedBlocks(r, numBlocks)
		baseHeight := chainIndexedBlocks[0].Height
		bestHeight := chainIndexedBlocks[len(chainIndexedBlocks)-1].Height
		ctl := gomock.NewController(t)
		mockBtcClient := mocks.NewMockBTCClient(ctl)
		confirmedBlocks := chainIndexedBlocks[:numBlocks-k]
		mockBtcClient.EXPECT().MustSubscribeBlocks().Return().AnyTimes()
		mockBtcClient.EXPECT().GetBestBlock().Return(nil, uint64(bestHeight), nil)
		for i := 0; i < int(numBlocks); i++ {
			mockBtcClient.EXPECT().GetBlockByHeight(gomock.Eq(uint64(chainIndexedBlocks[i].Height))).
				Return(chainIndexedBlocks[i], nil, nil).AnyTimes()
		}

		cache, err := types.NewBTCCache(numBlocks)
		require.NoError(t, err)
		btcScanner := &btcscanner.BtcScanner{
			BtcClient:             mockBtcClient,
			BaseHeight:            uint64(baseHeight),
			K:                     k,
			ConfirmedBlocksChan:   make(chan *types.IndexedBlock),
			UnconfirmedBlockCache: cache,
			Synced:                atomic.NewBool(false),
		}
		logger, err := config.NewRootLogger("auto", "debug")
		require.NoError(t, err)
		btcScanner.SetLogger(logger.Sugar())

		go func() {
			for i := 0; i < len(confirmedBlocks); i++ {
				b := <-btcScanner.ConfirmedBlocksChan
				require.Equal(t, confirmedBlocks[i].BlockHash(), b.BlockHash())
			}
		}()
		btcScanner.Bootstrap()
		require.True(t, btcScanner.Synced.Load())
	})
}
