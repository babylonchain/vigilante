package querier_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/rpc-client/testutil/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/vigilante/querier"
	"github.com/babylonchain/vigilante/types"
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
		bbnCli := mocks.NewMockBabylonQueryClient(ctrl)
		bbnCli.EXPECT().BlsPublicKeyList(gomock.Eq(e), gomock.Nil()).Return(
			&ckpttypes.QueryBlsPublicKeyListResponse{
				ValidatorWithBlsKeys: valSet.ValSet,
			},
			nil,
		).AnyTimes()
		bbnCli.EXPECT().RawCheckpoint(gomock.Eq(e)).Return(
			&ckpttypes.QueryRawCheckpointResponse{
				RawCheckpoint: ckptWithMeta,
			},
			nil,
		).AnyTimes()
		expectedEI := types.NewEpochInfo(e, *valSet)
		q := querier.New(bbnCli)
		ei, err := q.QueryInfoForNextEpoch(e)
		require.NoError(t, err)
		require.True(t, expectedEI.Equal(ei))
	})
}
