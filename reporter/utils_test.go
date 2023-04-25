package reporter_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	bbnmocks "github.com/babylonchain/rpc-client/testutil/mocks"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/reporter"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/babylonchain/vigilante/types"
)

func newMockReporter(t *testing.T, ctrl *gomock.Controller) (
	*mocks.MockBTCClient, *bbnmocks.MockBabylonClient, *reporter.Reporter) {
	cfg := config.DefaultConfig()

	mockBTCClient := mocks.NewMockBTCClient(ctrl)
	mockBabylonClient := bbnmocks.NewMockBabylonClient(ctrl)
	btccParams := btcctypes.DefaultParams()
	mockBabylonClient.EXPECT().GetConfig().Return(&cfg.Babylon).AnyTimes()
	mockBabylonClient.EXPECT().BTCCheckpointParams().Return(
		&btcctypes.QueryParamsResponse{Params: btccParams}, nil).AnyTimes()

	r, err := reporter.New(
		&cfg.Reporter,
		mockBTCClient,
		mockBabylonClient,
		cfg.Common.RetrySleepTime,
		cfg.Common.MaxRetrySleepTime,
		metrics.NewReporterMetrics(),
	)
	require.NoError(t, err)

	return mockBTCClient, mockBabylonClient, r
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
		numBlocks := datagen.RandomIntOtherThan(0, 100)
		blocks, _, _ := vdatagen.GenRandomBlockchainWithBabylonTx(numBlocks, 0, 0)
		ibs := []*types.IndexedBlock{}
		for _, block := range blocks {
			ibs = append(ibs, types.NewIndexedBlockFromMsgBlock(rand.Int31(), block))
		}

		_, mockBabylonClient, r := newMockReporter(t, ctrl)

		// a random number of blocks exists on chain
		numBlocksOnChain := rand.Intn(int(numBlocks))
		mockBabylonClient.EXPECT().ContainsBTCBlock(gomock.Any()).Return(
			&btclctypes.QueryContainsBytesResponse{Contains: true}, nil).Times(numBlocksOnChain)
		mockBabylonClient.EXPECT().ContainsBTCBlock(gomock.Any()).Return(
			&btclctypes.QueryContainsBytesResponse{Contains: false}, nil).AnyTimes()

		// inserting header will always be successful
		mockBabylonClient.EXPECT().InsertHeaders(gomock.Any()).Return(&sdk.TxResponse{Code: 0}, nil).AnyTimes()

		// if Babylon client contains this block, numSubmitted has to be 0, otherwise 1
		numSubmitted, err := r.ProcessHeaders(nil, ibs)
		require.Equal(t, int(numBlocks)-numBlocksOnChain, numSubmitted)
		require.NoError(t, err)
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

		_, mockBabylonClient, r := newMockReporter(t, ctrl)
		// inserting SPV proofs is always successful
		mockBabylonClient.EXPECT().InsertBTCSpvProof(gomock.Any()).Return(&sdk.TxResponse{Code: 0}, nil).AnyTimes()

		// generate a random number of blocks, with or without Babylon txs
		numBlocks := datagen.RandomInt(100)
		blocks, numCkptSegsExpected, rawCkpts := vdatagen.GenRandomBlockchainWithBabylonTx(numBlocks, 0.3, 0.4)
		ibs := []*types.IndexedBlock{}
		numMatchedCkptsExpected := 0
		for i, block := range blocks {
			ibs = append(ibs, types.NewIndexedBlockFromMsgBlock(rand.Int31(), block))
			if rawCkpts[i] != nil {
				numMatchedCkptsExpected++
			}
		}

		numCkptSegs, numMatchedCkpts, err := r.ProcessCheckpoints(nil, ibs)
		require.Equal(t, numCkptSegsExpected, numCkptSegs)
		require.Equal(t, numMatchedCkptsExpected, numMatchedCkpts)
		require.NoError(t, err)
	})
}
