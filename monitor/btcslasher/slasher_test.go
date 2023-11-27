package btcslasher_test

import (
	"bytes"
	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/btcstaking"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor/btcslasher"
	"github.com/babylonchain/vigilante/testutil/mocks"
)

func FuzzSlasher(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		net := &chaincfg.SimNetParams
		ctrl := gomock.NewController(t)

		mockBabylonQuerier := btcslasher.NewMockBabylonQueryClient(ctrl)
		mockBTCClient := mocks.NewMockBTCClient(ctrl)
		// mock k, w
		btccParams := &btcctypes.QueryParamsResponse{Params: btcctypes.Params{BtcConfirmationDepth: 10, CheckpointFinalizationTimeout: 100}}
		mockBabylonQuerier.EXPECT().BTCCheckpointParams().Return(btccParams, nil).Times(1)
		// covenant secret key
		covQuorum := datagen.RandomInt(r, 5) + 1
		covenantSks := make([]*btcec.PrivateKey, 0, covQuorum)
		covenantBtcPks := make([]*btcec.PublicKey, 0, covQuorum)
		covenantPks := make([]bbn.BIP340PubKey, 0, covQuorum)
		for idx := uint64(0); idx < covQuorum; idx++ {
			covenantSk, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			btcPubKey := covenantSk.PubKey()
			covenantSks = append(covenantSks, covenantSk)
			covenantBtcPks = append(covenantBtcPks, btcPubKey)
			covenantPks = append(covenantPks, *bbn.NewBIP340PubKeyFromBTCPK(btcPubKey))
		}
		// mock slashing rate and covenant
		bsParams := &bstypes.QueryParamsResponse{Params: bstypes.Params{
			// TODO: Can't use the below value as the datagen functionality only covers one covenant signature
			// CovenantQuorum: uint32(covQuorum),
			CovenantQuorum: 1,
			CovenantPks:    covenantPks,
			SlashingRate:   sdkmath.LegacyMustNewDecFromStr("0.1"),
		}}
		mockBabylonQuerier.EXPECT().BTCStakingParams().Return(bsParams, nil).Times(1)

		btcSlasher, err := btcslasher.New(mockBTCClient, mockBabylonQuerier, &chaincfg.SimNetParams, metrics.NewMonitorMetrics().SlasherMetrics)
		require.NoError(t, err)

		// mock chain tip
		randomBTCHeight := uint64(1000)
		mockBTCClient.EXPECT().GetBestBlock().Return(nil, randomBTCHeight, nil).Times(1)

		// slashing and change address
		slashingAddr, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)
		changeAddr, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)

		// generate BTC key pair for slashed BTC validator
		valSK, valPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		valBTCPK := bbn.NewBIP340PubKeyFromBTCPK(valPK)

		// mock a list of expired BTC delegations for this BTC validator
		expiredBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			delAmount := datagen.RandomInt(r, 100000) + 10000
			//  chain tip 1000 > end height - w 999, expired
			expiredBTCDel, err := datagen.GenRandomBTCDelegation(
				r,
				t,
				[]bbn.BIP340PubKey{*valBTCPK},
				delSK,
				covenantSks,
				bsParams.Params.CovenantQuorum,
				slashingAddr.String(),
				changeAddr.String(),
				100,
				1099,
				delAmount,
				bsParams.Params.SlashingRate,
			)
			require.NoError(t, err)
			expiredBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{expiredBTCDel}}
			expiredBTCDelsList = append(expiredBTCDelsList, expiredBTCDels)
		}
		// mock a list of BTC delegations whose timelocks are not expired for this BTC validator
		activeBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			delAmount := datagen.RandomInt(r, 100000) + 10000
			// start height 100 < chain tip 1000 == end height - w 1000, still active
			activeBTCDel, err := datagen.GenRandomBTCDelegation(
				r,
				t,
				[]bbn.BIP340PubKey{*valBTCPK},
				delSK,
				covenantSks,
				bsParams.Params.CovenantQuorum,
				slashingAddr.String(),
				changeAddr.String(),
				100,
				1100,
				delAmount,
				bsParams.Params.SlashingRate,
			)
			require.NoError(t, err)
			activeBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{activeBTCDel}}
			activeBTCDelsList = append(activeBTCDelsList, activeBTCDels)
		}
		// mock a list of unbonding BTC delegations
		unbondingBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			delAmount := datagen.RandomInt(r, 100000) + 10000
			// start height 100 < chain tip 1000 == end height - w 1000, still active
			unbondingBTCDel, err := datagen.GenRandomBTCDelegation(
				r,
				t,
				[]bbn.BIP340PubKey{*valBTCPK},
				delSK,
				covenantSks,
				bsParams.Params.CovenantQuorum,
				slashingAddr.String(),
				changeAddr.String(),
				100,
				1100,
				delAmount,
				bsParams.Params.SlashingRate,
			)
			require.NoError(t, err)
			// Get staking info for the delegation
			stakingInfo, err := btcstaking.BuildStakingInfo(
				unbondingBTCDel.BtcPk.MustToBTCPK(),
				[]*btcec.PublicKey{valBTCPK.MustToBTCPK()},
				covenantBtcPks,
				bsParams.Params.CovenantQuorum,
				unbondingBTCDel.GetStakingTime(),
				btcutil.Amount(unbondingBTCDel.TotalSat),
				net,
			)
			require.NoError(t, err)
			// Get the spend information for the unbonding path
			unbondingPathSpendInfo, err := stakingInfo.UnbondingPathSpendInfo()
			require.NoError(t, err)
			stakingMsgTx, err := bstypes.ParseBtcTx(unbondingBTCDel.StakingTx)
			require.NoError(t, err)
			stakingTxHash := stakingMsgTx.TxHash()
			outPoint := wire.NewOutPoint(&stakingTxHash, 0)
			unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingTx(
				r,
				t,
				net,
				delSK,
				[]*btcec.PublicKey{valPK},
				covenantBtcPks,
				bsParams.Params.CovenantQuorum,
				outPoint,
				1000,
				9000,
				slashingAddr.String(),
				changeAddr.String(),
				bsParams.Params.SlashingRate,
			)
			require.NoError(t, err)
			slashingPathSpendInfo, err := unbondingSlashingInfo.UnbondingInfo.SlashingPathSpendInfo()
			require.NoError(t, err)
			delSlashingSig, err := unbondingSlashingInfo.SlashingTx.Sign(
				unbondingSlashingInfo.UnbondingTx,
				0,
				slashingPathSpendInfo.RevealedLeaf.Script,
				delSK,
				net,
			)
			require.NoError(t, err)
			covenantUnbondingSigs := make([]*bstypes.SignatureInfo, 0, len(covenantSks))
			covenantSlashingSigs := make([]*bbn.BIP340Signature, 0, len(covenantSks))
			for idx, sk := range covenantSks {
				covenantSlashingSig, err := unbondingSlashingInfo.SlashingTx.Sign(
					unbondingSlashingInfo.UnbondingTx,
					0,
					slashingPathSpendInfo.RevealedLeaf.Script,
					sk,
					net,
				)
				require.NoError(t, err)
				covenantSlashingSigs = append(covenantSlashingSigs, covenantSlashingSig)
				covenantUnbondingSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
					unbondingSlashingInfo.UnbondingTx,
					stakingMsgTx,
					unbondingBTCDel.StakingOutputIdx,
					unbondingPathSpendInfo.RevealedLeaf.Script,
					sk,
					net,
				)
				require.NoError(t, err)

				covenantUnbondingSig := bbn.NewBIP340SignatureFromBTCSig(covenantUnbondingSchnorrSig)
				covenantUnbondingSigs = append(covenantUnbondingSigs, &bstypes.SignatureInfo{
					Pk:  &covenantPks[idx],
					Sig: &covenantUnbondingSig,
				})
			}
			// Convert the unbonding tx to bytes
			var unbondingTxBuffer bytes.Buffer
			err = unbondingSlashingInfo.UnbondingTx.Serialize(&unbondingTxBuffer)
			require.NoError(t, err)
			unbondingBTCDel.BtcUndelegation = &bstypes.BTCUndelegation{
				UnbondingTx:          unbondingTxBuffer.Bytes(),
				SlashingTx:           unbondingSlashingInfo.SlashingTx,
				DelegatorSlashingSig: delSlashingSig,
				// TODO: currently requires only one sig, in reality requires all of them
				CovenantSlashingSig:      covenantSlashingSigs[0],
				CovenantUnbondingSigList: covenantUnbondingSigs,
			}
			// append
			unbondingBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{unbondingBTCDel}}
			unbondingBTCDelsList = append(unbondingBTCDelsList, unbondingBTCDels)
		}

		// mock query to BTCValidatorDelegations
		dels := []*bstypes.BTCDelegatorDelegations{}
		dels = append(dels, expiredBTCDelsList...)
		dels = append(dels, activeBTCDelsList...)
		dels = append(dels, unbondingBTCDelsList...)
		btcDelsResp := &bstypes.QueryBTCValidatorDelegationsResponse{
			BtcDelegatorDelegations: dels,
			Pagination:              &query.PageResponse{NextKey: nil},
		}
		mockBabylonQuerier.EXPECT().BTCValidatorDelegations(gomock.Eq(valBTCPK.MarshalHex()), gomock.Any()).Return(btcDelsResp, nil).Times(1)

		// mock GetTxOut called for each BTC undelegation
		mockBTCClient.EXPECT().
			GetTxOut(gomock.Any(), gomock.Any(), gomock.Eq(true)).
			Return(&btcjson.GetTxOutResult{}, nil).
			Times(len(unbondingBTCDelsList))

		// ensure there should be only len(activeBTCDelsList) + len(unbondingBTCDelsList) BTC txs
		mockBTCClient.EXPECT().
			SendRawTransaction(gomock.Any(), gomock.Eq(true)).
			Return(&chainhash.Hash{}, nil).
			Times(len(activeBTCDelsList) + len(unbondingBTCDelsList))

		err = btcSlasher.SlashBTCValidator(valBTCPK, valSK, false)
		require.NoError(t, err)
	})
}
