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

// FuzzProcessHeaders is a fuzz tests for ProcessHeaders()
// - Data: a number of random blocks, with or without Babylon txs
// - Tested property: for any BTC block, if its header is not duplicated, then it will submit this header
func FuzzProcessHeaders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		rand.Seed(seed)

		containsBlock := datagen.OneInN(2)

		block, _ := vdatagen.GenRandomBlock(false, nil)
		ib := types.NewIndexedBlockFromMsgBlock(rand.Int31(), block)

		_, mockBabylonClient, reporter := newMockReporter(t, ctrl)

		// may or may not contain this block
		mockBabylonClient.EXPECT().QueryContainsBlock(gomock.Any()).Return(containsBlock, nil).AnyTimes()
		// inserting header will always be successful
		mockBabylonClient.EXPECT().InsertHeaders(gomock.Any()).Return(&sdk.TxResponse{Code: 0}, nil).AnyTimes()

		// if Babylon client contains this block, numSubmitted has to be 0, otherwise 1
		numSubmitted := reporter.ProcessHeaders(nil, []*types.IndexedBlock{ib})
		if containsBlock {
			require.Equal(t, 0, numSubmitted)
		} else {
			require.Equal(t, 1, numSubmitted)
		}

	})
}

// FuzzProcessCheckpoints is a fuzz tests for ProcessCheckpoints()
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
		containsCkpt = true
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
