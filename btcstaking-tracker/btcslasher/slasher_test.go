package btcslasher_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/btcstaking"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/vigilante/btcstaking-tracker/btcslasher"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/testutil/mocks"
)

func FuzzSlasher(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		net := &chaincfg.SimNetParams
		commonCfg := config.DefaultCommonConfig()
		ctrl := gomock.NewController(t)

		mockBabylonQuerier := btcslasher.NewMockBabylonQueryClient(ctrl)
		mockBTCClient := mocks.NewMockBTCClient(ctrl)
		// mock k, w
		btccParams := &btcctypes.QueryParamsResponse{Params: btcctypes.Params{BtcConfirmationDepth: 10, CheckpointFinalizationTimeout: 100}}
		mockBabylonQuerier.EXPECT().BTCCheckpointParams().Return(btccParams, nil).Times(1)
		unbondingTime := uint16(btccParams.Params.CheckpointFinalizationTimeout + 1)

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

		logger, err := config.NewRootLogger("auto", "debug")
		require.NoError(t, err)
		slashedFPSKChan := make(chan *btcec.PrivateKey, 100)
		btcSlasher, err := btcslasher.New(logger, mockBTCClient, mockBabylonQuerier, &chaincfg.SimNetParams, commonCfg.RetrySleepTime, commonCfg.MaxRetrySleepTime, slashedFPSKChan, metrics.NewBTCStakingTrackerMetrics().SlasherMetrics)
		require.NoError(t, err)
		err = btcSlasher.LoadParams()
		require.NoError(t, err)

		// mock chain tip
		randomBTCHeight := uint64(1000)
		mockBTCClient.EXPECT().GetBestBlock().Return(nil, randomBTCHeight, nil).Times(1)

		// slashing and change address
		slashingAddr, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)

		// generate BTC key pair for slashed finality provider
		valSK, valPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(valPK)

		// mock a list of expired BTC delegations for this finality provider
		expiredBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			delAmount := datagen.RandomInt(r, 100000) + 10000
			//  chain tip 1000 > end height - w 999, expired
			expiredBTCDel, err := datagen.GenRandomBTCDelegation(
				r,
				t,
				[]bbn.BIP340PubKey{*fpBTCPK},
				delSK,
				covenantSks,
				bsParams.Params.CovenantQuorum,
				slashingAddr.String(),
				100,
				1099,
				delAmount,
				bsParams.Params.SlashingRate,
				unbondingTime,
			)
			require.NoError(t, err)
			expiredBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{expiredBTCDel}}
			expiredBTCDelsList = append(expiredBTCDelsList, expiredBTCDels)
		}
		// mock a list of BTC delegations whose timelocks are not expired for this finality provider
		activeBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			delAmount := datagen.RandomInt(r, 100000) + 10000
			// start height 100 < chain tip 1000 == end height - w 1000, still active
			activeBTCDel, err := datagen.GenRandomBTCDelegation(
				r,
				t,
				[]bbn.BIP340PubKey{*fpBTCPK},
				delSK,
				covenantSks,
				bsParams.Params.CovenantQuorum,
				slashingAddr.String(),
				100,
				1100,
				delAmount,
				bsParams.Params.SlashingRate,
				unbondingTime,
			)
			require.NoError(t, err)
			activeBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{activeBTCDel}}
			activeBTCDelsList = append(activeBTCDelsList, activeBTCDels)
		}
		// mock a list of unbonded BTC delegations
		unbondedBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			delAmount := datagen.RandomInt(r, 100000) + 10000
			// start height 100 < chain tip 1000 == end height - w 1000, still active
			unbondingBTCDel, err := datagen.GenRandomBTCDelegation(
				r,
				t,
				[]bbn.BIP340PubKey{*fpBTCPK},
				delSK,
				covenantSks,
				bsParams.Params.CovenantQuorum,
				slashingAddr.String(),
				100,
				1100,
				delAmount,
				bsParams.Params.SlashingRate,
				unbondingTime,
			)
			require.NoError(t, err)
			// Get staking info for the delegation
			stakingInfo, err := btcstaking.BuildStakingInfo(
				unbondingBTCDel.BtcPk.MustToBTCPK(),
				[]*btcec.PublicKey{fpBTCPK.MustToBTCPK()},
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
			stakingMsgTx, err := bbn.NewBTCTxFromBytes(unbondingBTCDel.StakingTx)
			require.NoError(t, err)
			stakingTxHash := stakingMsgTx.TxHash()
			outPoint := wire.NewOutPoint(&stakingTxHash, 0)
			unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
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
				bsParams.Params.SlashingRate,
				unbondingTime,
			)
			require.NoError(t, err)
			slashingPathSpendInfo, err := unbondingSlashingInfo.UnbondingInfo.SlashingPathSpendInfo()
			require.NoError(t, err)
			delSlashingSig, err := unbondingSlashingInfo.SlashingTx.Sign(
				unbondingSlashingInfo.UnbondingTx,
				0,
				slashingPathSpendInfo.GetPkScriptPath(),
				delSK,
			)
			require.NoError(t, err)
			covenantUnbondingSigs := make([]*bstypes.SignatureInfo, 0, len(covenantSks))
			covenantSlashingSigs := make([]*bstypes.CovenantAdaptorSignatures, 0, len(covenantSks))
			for idx, sk := range covenantSks {
				// covenant adaptor signature on slashing tx
				encKey, err := asig.NewEncryptionKeyFromBTCPK(valPK)
				require.NoError(t, err)
				covenantSlashingSig, err := unbondingSlashingInfo.SlashingTx.EncSign(
					unbondingSlashingInfo.UnbondingTx,
					0,
					slashingPathSpendInfo.GetPkScriptPath(),
					sk,
					encKey,
				)
				require.NoError(t, err)
				covenantSlashingSigs = append(covenantSlashingSigs, &bstypes.CovenantAdaptorSignatures{
					CovPk:       bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
					AdaptorSigs: [][]byte{covenantSlashingSig.MustMarshal()},
				})
				// covenant Schnorr signature on unbonding tx
				covenantUnbondingSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
					unbondingSlashingInfo.UnbondingTx,
					stakingMsgTx,
					unbondingBTCDel.StakingOutputIdx,
					unbondingPathSpendInfo.GetPkScriptPath(),
					sk,
				)
				require.NoError(t, err)

				covenantUnbondingSig := bbn.NewBIP340SignatureFromBTCSig(covenantUnbondingSchnorrSig)
				covenantUnbondingSigs = append(covenantUnbondingSigs, &bstypes.SignatureInfo{
					Pk:  &covenantPks[idx],
					Sig: covenantUnbondingSig,
				})
			}
			// Convert the unbonding tx to bytes
			var unbondingTxBuffer bytes.Buffer
			err = unbondingSlashingInfo.UnbondingTx.Serialize(&unbondingTxBuffer)
			require.NoError(t, err)
			unbondingBTCDel.BtcUndelegation = &bstypes.BTCUndelegation{
				UnbondingTx:           unbondingTxBuffer.Bytes(),
				SlashingTx:            unbondingSlashingInfo.SlashingTx,
				DelegatorSlashingSig:  delSlashingSig,
				DelegatorUnbondingSig: delSlashingSig,
				// TODO: currently requires only one sig, in reality requires all of them
				CovenantSlashingSigs:     covenantSlashingSigs,
				CovenantUnbondingSigList: covenantUnbondingSigs,
			}
			// append
			unbondingBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{unbondingBTCDel}}
			unbondedBTCDelsList = append(unbondedBTCDelsList, unbondingBTCDels)
		}

		// mock query to FinalityProviderDelegations
		dels := []*bstypes.BTCDelegatorDelegations{}
		dels = append(dels, expiredBTCDelsList...)
		dels = append(dels, activeBTCDelsList...)
		dels = append(dels, unbondedBTCDelsList...)
		btcDelsResp := &bstypes.QueryFinalityProviderDelegationsResponse{
			BtcDelegatorDelegations: dels,
			Pagination:              &query.PageResponse{NextKey: nil},
		}
		mockBabylonQuerier.EXPECT().FinalityProviderDelegations(gomock.Eq(fpBTCPK.MarshalHex()), gomock.Any()).Return(btcDelsResp, nil).Times(1)

		mockBTCClient.EXPECT().
			GetRawTransaction(gomock.Any()).
			Return(nil, fmt.Errorf("tx does not exist")).
			Times((len(activeBTCDelsList) + len(unbondedBTCDelsList)) * 2)

		mockBTCClient.EXPECT().
			GetTxOut(gomock.Any(), gomock.Any(), gomock.Eq(true)).
			Return(&btcjson.GetTxOutResult{}, nil).
			Times((len(activeBTCDelsList) + len(unbondedBTCDelsList)) * 2)

		mockBTCClient.EXPECT().
			SendRawTransaction(gomock.Any(), gomock.Eq(true)).
			Return(&chainhash.Hash{}, nil).
			Times((len(activeBTCDelsList) + len(unbondedBTCDelsList)) * 2)

		err = btcSlasher.SlashFinalityProvider(valSK)
		require.NoError(t, err)

		btcSlasher.WaitForShutdown()
	})
}
