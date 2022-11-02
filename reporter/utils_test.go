package reporter_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/reporter"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/babylonchain/vigilante/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func newMockReporter(t *testing.T, ctrl *gomock.Controller) (*mocks.MockBTCClient, *mocks.MockBabylonClient, *reporter.Reporter) {
	cfg := config.DefaultConfig()

	mockBTCClient := mocks.NewMockBTCClient(ctrl)
	mockBabylonClient := mocks.NewMockBabylonClient(ctrl)
	btccParams := btcctypes.DefaultParams()
	mockBabylonClient.EXPECT().MustQueryBTCCheckpointParams().Return(&btccParams).AnyTimes()
	mockBabylonClient.EXPECT().GetTagIdx().Return(uint8(48)).AnyTimes()
	mockBabylonClient.EXPECT().GetConfig().Return(&cfg.Babylon).AnyTimes()

	reporter, err := reporter.New(
		&cfg.Reporter,
		mockBTCClient,
		mockBabylonClient,
		cfg.Common.RetrySleepTime,
		cfg.Common.MaxRetrySleepTime,
	)
	require.NoError(t, err)

	return mockBTCClient, mockBabylonClient, reporter
}

// FuzzProcessHeaders fuzz tests ProcessHeaders()
// - Data: a number of random blocks, with or without Babylon txs
// - Tested property: for any BTC block, if its header is not duplicated, then it will submit this header
func FuzzProcessHeaders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		rand.Seed(seed)

		// generate a random number of blocks
		numBlocks := datagen.RandomInt(100)
		blocks, _ := vdatagen.GenRandomBlockchainWithBabylonTx(numBlocks, 0)
		ibs := []*types.IndexedBlock{}
		for _, block := range blocks {
			ibs = append(ibs, types.NewIndexedBlockFromMsgBlock(rand.Int31(), block))
		}

		_, mockBabylonClient, reporter := newMockReporter(t, ctrl)

		// a random number of blocks exists on chain
		numBlocksOnChain := rand.Intn(int(numBlocks))
		mockBabylonClient.EXPECT().QueryContainsBlock(gomock.Any()).Return(true, nil).Times(numBlocksOnChain)
		mockBabylonClient.EXPECT().QueryContainsBlock(gomock.Any()).Return(false, nil).AnyTimes()

		// inserting header will always be successful
		mockBabylonClient.EXPECT().InsertHeaders(gomock.Any()).Return(&sdk.TxResponse{Code: 0}, nil).AnyTimes()

		// if Babylon client contains this block, numSubmitted has to be 0, otherwise 1
		numSubmitted := reporter.ProcessHeaders(nil, ibs)
		require.Equal(t, int(numBlocks)-numBlocksOnChain, numSubmitted)
	})
}

// FuzzProcessCheckpoints fuzz tests ProcessCheckpoints()
// - Data: a number of random blocks, with or without Babylon txs
// - Tested property: for any BTC block, if it contains Babylon data, then it will extract checkpoint segments, do a match, and report matched checkpoints
func FuzzProcessCheckpoints(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		rand.Seed(seed)

		_, mockBabylonClient, reporter := newMockReporter(t, ctrl)
		// inserting SPV proofs is always successful
		mockBabylonClient.EXPECT().MustInsertBTCSpvProof(gomock.Any()).Return(&sdk.TxResponse{Code: 0}).AnyTimes()

		containsCkpt := datagen.OneInN(2)
		block, _ := vdatagen.GenRandomBlock(containsCkpt, nil)
		ib := types.NewIndexedBlockFromMsgBlock(rand.Int31(), block)

		numCkptSegs, numMatchedCkpts := reporter.ProcessCheckpoints(nil, []*types.IndexedBlock{ib})
		if containsCkpt {
			require.Equal(t, 2, numCkptSegs)
			require.Equal(t, 1, numMatchedCkpts)
		} else {
			require.Equal(t, 0, numCkptSegs)
			require.Equal(t, 0, numMatchedCkpts)
		}
	})
}
