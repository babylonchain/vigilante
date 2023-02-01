package querier_test

import (
	"github.com/babylonchain/babylon/testutil/datagen"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/rpc-client/testutil/mocks"
	"github.com/babylonchain/vigilante/monitor/querier"
	"github.com/babylonchain/vigilante/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

// FuzzQueryInfoForNextEpoch generates validator set with BLS keys and raw checkpoints
// and check whether they are the same as the queried epoch info
func FuzzQueryInfoForNextEpoch(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		n := rand.Intn(100) + 1
		valSet, blsprivkeys := datagen.GenerateValidatorSetWithBLSPrivKeys(n)
		ckpt := datagen.GenerateLegitimateRawCheckpoint(blsprivkeys)
		e := ckpt.EpochNum
		ckptWithMeta := &ckpttypes.RawCheckpointWithMeta{Ckpt: ckpt}
		ctrl := gomock.NewController(t)
		bbnCli := mocks.NewMockBabylonClient(ctrl)
		bbnCli.EXPECT().BlsPublicKeyList(gomock.Eq(e)).Return(valSet.ValSet, nil).AnyTimes()
		bbnCli.EXPECT().QueryRawCheckpoint(gomock.Eq(e)).Return(ckptWithMeta, nil).AnyTimes()
		expectedEI := types.NewEpochInfo(e, *valSet)
		q := querier.New(bbnCli)
		ei, err := q.QueryInfoForNextEpoch(e)
		require.NoError(t, err)
		require.True(t, expectedEI.Equal(ei))
	})
}
