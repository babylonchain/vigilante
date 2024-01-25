package monitor_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/crypto/bls12381"
	"github.com/babylonchain/babylon/testutil/datagen"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func GetMsgBytes(epoch uint64, hash *ckpttypes.BlockHash) []byte {
	return append(sdk.Uint64ToBigEndian(epoch), hash.MustMarshal()...)
}

type TestCase struct {
	name            string
	btcCheckpoint   *ckpttypes.RawCheckpoint
	expectNilErr    bool
	expectInconsist bool
}

func FuzzVerifyCheckpoint(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		var testCases []*TestCase

		ctl := gomock.NewController(t)
		mockBabylonClient := monitor.NewMockBabylonQueryClient(ctl)
		m := &monitor.Monitor{
			BBNQuerier: mockBabylonClient,
		}

		// at least 4 validators
		n := r.Intn(10) + 4
		valSet, privKeys := datagen.GenerateValidatorSetWithBLSPrivKeys(n)
		btcCheckpoint := datagen.GenerateLegitimateRawCheckpoint(r, privKeys)
		mockBabylonClient.EXPECT().RawCheckpoint(gomock.Eq(btcCheckpoint.EpochNum)).Return(
			&ckpttypes.QueryRawCheckpointResponse{
				RawCheckpoint: &ckpttypes.RawCheckpointWithMeta{
					Ckpt: btcCheckpoint,
				},
			}, nil).AnyTimes()
		// generate case 1, same checkpoints
		case1 := &TestCase{
			name:            "valid checkpoint",
			btcCheckpoint:   btcCheckpoint,
			expectNilErr:    true,
			expectInconsist: false,
		}
		testCases = append(testCases, case1)

		// generate case 2, using invalid multi-sig
		btcCheckpoint2 := &ckpttypes.RawCheckpoint{}
		err := copier.Copy(btcCheckpoint2, btcCheckpoint)
		require.NoError(t, err)
		sig := datagen.GenRandomBlsMultiSig(r)
		btcCheckpoint2.BlsMultiSig = &sig
		case2 := &TestCase{
			name:            "invalid multi-sig",
			btcCheckpoint:   btcCheckpoint2,
			expectNilErr:    false,
			expectInconsist: false,
		}
		testCases = append(testCases, case2)

		// generate case 3, using invalid epoch num
		newEpoch := datagen.GenRandomEpochNum(r)
		for {
			if newEpoch != btcCheckpoint2.EpochNum {
				break
			}
			newEpoch = datagen.GenRandomEpochNum(r)
		}
		btcCheckpoint3 := &ckpttypes.RawCheckpoint{}
		err = copier.Copy(btcCheckpoint3, btcCheckpoint)
		require.NoError(t, err)
		btcCheckpoint3.EpochNum = newEpoch
		case3 := &TestCase{
			name:            "invalid epoch num",
			btcCheckpoint:   btcCheckpoint3,
			expectNilErr:    false,
			expectInconsist: false,
		}
		testCases = append(testCases, case3)

		// generate case 4, using different BlockHash
		btcCheckpoint4 := &ckpttypes.RawCheckpoint{}
		err = copier.Copy(btcCheckpoint4, btcCheckpoint)
		require.NoError(t, err)
		blockHash2 := datagen.GenRandomBlockHash(r)
		msgBytes2 := GetMsgBytes(btcCheckpoint4.EpochNum, &blockHash2)
		signerNum := n*2/3 + 1
		sigs2 := datagen.GenerateBLSSigs(privKeys[:signerNum], msgBytes2)
		multiSig2, err := bls12381.AggrSigList(sigs2)
		require.NoError(t, err)
		btcCheckpoint4.BlockHash = &blockHash2
		btcCheckpoint4.BlsMultiSig = &multiSig2
		case4 := &TestCase{
			name:            "fork found",
			btcCheckpoint:   btcCheckpoint4,
			expectNilErr:    false,
			expectInconsist: true,
		}
		testCases = append(testCases, case4)

		for _, tc := range testCases {
			mockBabylonClient.EXPECT().BlsPublicKeyList(gomock.Eq(tc.btcCheckpoint.EpochNum), gomock.Nil()).Return(
				&ckpttypes.QueryBlsPublicKeyListResponse{
					ValidatorWithBlsKeys: valSet.ValSet,
				}, nil).AnyTimes()
			err := m.UpdateEpochInfo(btcCheckpoint.EpochNum)
			require.NoError(t, err)
			err = m.VerifyCheckpoint(tc.btcCheckpoint)
			if tc.expectNilErr {
				require.NoError(t, err, "error at test case %s", tc.name)
			}
			if tc.expectInconsist {
				require.ErrorIs(t, err, types.ErrInconsistentBlockHash)
			}
		}
	})
}
