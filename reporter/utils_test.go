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

func newMockReporter(t *testing.T) (*mocks.MockBTCClient, *mocks.MockBabylonClient, *reporter.Reporter) {
	cfg := config.DefaultConfig()

	mockBTCClient := mocks.NewMockBTCClient(gomock.NewController(t))

	mockBabylonClient := mocks.NewMockBabylonClient(gomock.NewController(t))
	btccParams := btcctypes.DefaultParams()
	mockBabylonClient.EXPECT().MustQueryBTCCheckpointParams().Return(&btccParams)
	mockBabylonClient.EXPECT().GetTagIdx().Return(uint8(48))

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

// TODO: tests on handler functions
// - given a BTC block with/without OP_RETURN txs/signer, test parsing/extracting ckpts in this block
// - given a BTC block, test parsing/extracting header in this block

// FuzzProcessHeaders is a fuzz tests for ProcessHeaders()
// - Data: a number of random blocks, with or without Babylon txs
// - Tested property: for any BTC block, if its header is not duplicated, then it will submit this header
func FuzzProcessHeaders(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		containsBlock := datagen.OneInN(2)

		block, _ := vdatagen.GenRandomBlock(false, nil)
		ib := types.NewIndexedBlockFromMsgBlock(rand.Int31(), block)

		_, mockBabylonClient, reporter := newMockReporter(t)

		mockBabylonClient.EXPECT().QueryContainsBlock(gomock.Any()).Return(containsBlock, nil)
		mockBabylonClient.EXPECT().InsertHeaders(gomock.Any()).Return(&sdk.TxResponse{Code: 0}, nil)

		// if Babylon client contains this block, numSubmitted has to be 0, otherwise 1
		numSubmitted := reporter.ProcessHeaders(nil, []*types.IndexedBlock{ib})
		if containsBlock {
			require.Equal(t, 1, numSubmitted)
		} else {
			require.Equal(t, 0, numSubmitted)
		}
	})
}
