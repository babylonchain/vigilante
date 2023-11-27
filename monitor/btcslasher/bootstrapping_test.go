package btcslasher_test

import (
	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"math/rand"
	"testing"

	datagen "github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor/btcslasher"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func FuzzSlasher_Bootstrapping(f *testing.F) {
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
		covenantPks := make([]bbn.BIP340PubKey, 0, covQuorum)
		for idx := uint64(0); idx < covQuorum; idx++ {
			covenantSk, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			covenantSks = append(covenantSks, covenantSk)
			covenantPks = append(covenantPks, *bbn.NewBIP340PubKeyFromBTCPK(covenantSk.PubKey()))
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
		// mock an evidence with this BTC validator
		evidence, err := datagen.GenRandomEvidence(r, valSK, 100)
		require.NoError(t, err)
		mockBabylonQuerier.EXPECT().ListEvidences(gomock.Any(), gomock.Any()).Return(&ftypes.QueryListEvidencesResponse{
			Evidences:  []*ftypes.Evidence{evidence},
			Pagination: &query.PageResponse{NextKey: nil},
		}, nil).Times(1)

		// mock a list of active BTC delegations whose staking tx's 2nd output is still spendable on Bitocin
		slashableBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
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
			slashableBTCDelsList = append(slashableBTCDelsList, activeBTCDels)
			// mock the BTC delegation's staking tx output is still slashable on Bitcoin
			tx, err := bstypes.ParseBtcTx(activeBTCDel.StakingTx)
			require.NoError(t, err)
			txHash := tx.TxHash()
			mockBTCClient.EXPECT().GetTxOut(gomock.Eq(&txHash), gomock.Eq(activeBTCDel.StakingOutputIdx), gomock.Eq(true)).Return(&btcjson.GetTxOutResult{}, nil).Times(1)
		}

		// mock a set of activeBTCDelsList whose staking tx's 2nd output is no longer spendable on Bitcoin
		unslashableBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
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
			unslashableBTCDelsList = append(unslashableBTCDelsList, activeBTCDels)
			// mock the BTC delegation's staking tx output is no longer slashable on Bitocin
			tx, err := bstypes.ParseBtcTx(activeBTCDel.StakingTx)
			require.NoError(t, err)
			txHash := tx.TxHash()
			mockBTCClient.EXPECT().GetTxOut(gomock.Eq(&txHash), gomock.Eq(activeBTCDel.StakingOutputIdx), gomock.Eq(true)).Return(nil, nil).Times(1)
		}

		// mock query to BTCValidatorDelegations
		btcDelsResp := &bstypes.QueryBTCValidatorDelegationsResponse{
			BtcDelegatorDelegations: append(slashableBTCDelsList, unslashableBTCDelsList...),
			Pagination:              &query.PageResponse{NextKey: nil},
		}
		mockBabylonQuerier.EXPECT().BTCValidatorDelegations(gomock.Eq(valBTCPK.MarshalHex()), gomock.Any()).Return(btcDelsResp, nil).Times(1)

		// ensure there should be only len(activeBTCDelsList) BTC txs
		mockBTCClient.EXPECT().
			SendRawTransaction(gomock.Any(), gomock.Eq(true)).
			Return(&chainhash.Hash{}, nil).
			Times(len(slashableBTCDelsList))

		err = btcSlasher.Bootstrap(0)
		require.NoError(t, err)
	})
}
