package btcslasher_test

import (
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

		btcSlasher, err := btcslasher.New(mockBTCClient, mockBabylonQuerier, &chaincfg.SimNetParams, metrics.NewMonitorMetrics().SlasherMetrics)
		require.NoError(t, err)

		// mock chain tip
		randomBTCHeight := uint64(1000)
		mockBTCClient.EXPECT().GetBestBlock().Return(nil, randomBTCHeight, nil).Times(1)

		// jury and slashing addr
		jurySK, juryPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		slashingAddr, err := datagen.GenRandomBTCAddress(r, net)
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
			//  chain tip 1000 > end height - w 999, expired
			expiredBTCDel, err := datagen.GenRandomBTCDelegation(r, valBTCPK, delSK, jurySK, slashingAddr.String(), 100, 1099, 10000)
			require.NoError(t, err)
			expiredBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{expiredBTCDel}}
			expiredBTCDelsList = append(expiredBTCDelsList, expiredBTCDels)
		}
		// mock a list of BTC delegations whose timelocks are not expired for this BTC validator
		activeBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			// start height 100 < chain tip 1000 == end height - w 1000, still active
			activeBTCDel, err := datagen.GenRandomBTCDelegation(r, valBTCPK, delSK, jurySK, slashingAddr.String(), 100, 1100, 10000)
			require.NoError(t, err)
			activeBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{activeBTCDel}}
			activeBTCDelsList = append(activeBTCDelsList, activeBTCDels)
		}
		// mock a list of unbonding BTC delegations
		unbondingBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			// start height 100 < chain tip 1000 == end height - w 1000, still active
			unbondingBTCDel, err := datagen.GenRandomBTCDelegation(r, valBTCPK, delSK, jurySK, slashingAddr.String(), 100, 1100, 10000)
			require.NoError(t, err)
			// create undelegation
			stakingMsgTx, err := unbondingBTCDel.StakingTx.ToMsgTx()
			require.NoError(t, err)
			stakingTxHash := stakingMsgTx.TxHash()
			outPoint := wire.NewOutPoint(&stakingTxHash, 0)
			unbondingTx, unbondingSlashingTx, err := datagen.GenBTCUnbondingSlashingTx(r, net, delSK, valPK, juryPK, outPoint, 1000, 9000, slashingAddr.String())
			require.NoError(t, err)
			unbondingMsgTx, err := unbondingTx.ToMsgTx()
			require.NoError(t, err)
			delSlashingSig, err := unbondingSlashingTx.Sign(unbondingMsgTx, unbondingTx.Script, delSK, net)
			require.NoError(t, err)
			jurySlashingSig, err := unbondingSlashingTx.Sign(unbondingMsgTx, unbondingTx.Script, jurySK, net)
			require.NoError(t, err)
			juryUnbondingSig, err := unbondingTx.Sign(stakingMsgTx, unbondingBTCDel.StakingTx.Script, jurySK, net)
			require.NoError(t, err)
			valUnbondingSig, err := unbondingTx.Sign(stakingMsgTx, unbondingBTCDel.StakingTx.Script, valSK, net)
			require.NoError(t, err)
			unbondingBTCDel.BtcUndelegation = &bstypes.BTCUndelegation{
				UnbondingTx:           unbondingTx,
				SlashingTx:            unbondingSlashingTx,
				DelegatorSlashingSig:  delSlashingSig,
				CovenantSlashingSig:   jurySlashingSig,
				CovenantUnbondingSig:  juryUnbondingSig,
				ValidatorUnbondingSig: valUnbondingSig,
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
