package types_test

import (
	"github.com/babylonchain/babylon/crypto/bls12381"
	"github.com/babylonchain/babylon/testutil/datagen"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

type TestCase struct {
	name            string
	ei              *types.EpochInfo
	btcCheckpoint   *ckpttypes.RawCheckpoint
	expectNilErr    bool
	expectInconsist bool
}

func FuzzVerifyCheckpoint(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)
		var testCases []*TestCase

		// at least 4 validators
		n := rand.Intn(10) + 4
		valSet, privKeys := datagen.GenerateValidatorSetWithBLSPrivKeys(n)
		btcCheckpoint := datagen.GenerateLegitimateRawCheckpoint(privKeys)
		// fix ei and change btcCheckpoint for each case
		ei := types.NewEpochInfo(btcCheckpoint.EpochNum, *valSet, btcCheckpoint)
		// generate case 1, same checkpoints
		case1 := &TestCase{
			name:            "valid checkpoint",
			ei:              ei,
			btcCheckpoint:   btcCheckpoint,
			expectNilErr:    true,
			expectInconsist: false,
		}
		testCases = append(testCases, case1)

		// generate case 2, using invalid multi-sig
		btcCheckpoint2 := &ckpttypes.RawCheckpoint{}
		err := copier.Copy(btcCheckpoint2, btcCheckpoint)
		require.NoError(t, err)
		sig := datagen.GenRandomBlsMultiSig()
		btcCheckpoint2.BlsMultiSig = &sig
		case2 := &TestCase{
			name:            "invalid multi-sig",
			ei:              ei,
			btcCheckpoint:   btcCheckpoint2,
			expectNilErr:    false,
			expectInconsist: false,
		}
		testCases = append(testCases, case2)

		// generate case 3, using invalid epoch num
		newEpoch := datagen.GenRandomEpochNum()
		for {
			if newEpoch != btcCheckpoint2.EpochNum {
				break
			}
			newEpoch = datagen.GenRandomEpochNum()
		}
		btcCheckpoint3 := &ckpttypes.RawCheckpoint{}
		err = copier.Copy(btcCheckpoint3, btcCheckpoint)
		require.NoError(t, err)
		btcCheckpoint3.EpochNum = newEpoch
		case3 := &TestCase{
			name:            "invalid epoch num",
			ei:              ei,
			btcCheckpoint:   btcCheckpoint3,
			expectNilErr:    false,
			expectInconsist: false,
		}
		testCases = append(testCases, case3)

		// generate case 4, using different lastCommitHash
		btcCheckpoint4 := &ckpttypes.RawCheckpoint{}
		err = copier.Copy(btcCheckpoint4, btcCheckpoint)
		require.NoError(t, err)
		lch2 := datagen.GenRandomLastCommitHash()
		msgBytes2 := types.GetMsgBytes(btcCheckpoint4.EpochNum, &lch2)
		signerNum := n/3 + 1
		sigs2 := datagen.GenerateBLSSigs(privKeys[:signerNum], msgBytes2)
		multiSig2, err := bls12381.AggrSigList(sigs2)
		require.NoError(t, err)
		btcCheckpoint4.LastCommitHash = &lch2
		btcCheckpoint4.BlsMultiSig = &multiSig2
		case4 := &TestCase{
			name:            "fork found",
			ei:              ei,
			btcCheckpoint:   btcCheckpoint4,
			expectNilErr:    false,
			expectInconsist: true,
		}
		testCases = append(testCases, case4)

		for _, tc := range testCases {
			err := tc.ei.VerifyCheckpoint(tc.btcCheckpoint)
			if tc.expectNilErr {
				require.NoError(t, err)
			}
			if tc.expectInconsist {
				require.ErrorIs(t, err, types.ErrInconsistentLastCommitHash)
			}
		}
	})
}
