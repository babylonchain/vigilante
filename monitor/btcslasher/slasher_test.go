package btcslasher_test

import (
	"math/rand"
	"testing"

	datagen "github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/vigilante/monitor/btcslasher"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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

		btcSlasher, err := btcslasher.New(mockBTCClient, mockBabylonQuerier, &chaincfg.SimNetParams)
		require.NoError(t, err)

		// mock chain tip
		randomBTCHeight := uint64(1000)
		mockBTCClient.EXPECT().GetBestBlock().Return(nil, randomBTCHeight, nil).Times(1)

		// jury and slashing addr
		jurySK, _, err := datagen.GenRandomBTCKeyPair(r)
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
			expiredBTCDel, err := datagen.GenRandomBTCDelegation(r, valBTCPK, delSK, jurySK, slashingAddr, 100, 899, 10000)
			require.NoError(t, err)
			expiredBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{expiredBTCDel}}
			expiredBTCDelsList = append(expiredBTCDelsList, expiredBTCDels)
		}
		// mock a list of BTC delegations whose timelocks are not expired for this BTC validator
		slashableBTCDelsList := []*bstypes.BTCDelegatorDelegations{}
		for i := uint64(0); i < datagen.RandomInt(r, 30)+5; i++ {
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			require.NoError(t, err)
			slashableBTCDel, err := datagen.GenRandomBTCDelegation(r, valBTCPK, delSK, jurySK, slashingAddr, 100, 9999, 10000)
			require.NoError(t, err)
			slashableBTCDels := &bstypes.BTCDelegatorDelegations{Dels: []*bstypes.BTCDelegation{slashableBTCDel}}
			slashableBTCDelsList = append(slashableBTCDelsList, slashableBTCDels)
		}

		// mock query to BTCValidatorDelegations
		btcDelsResp := &bstypes.QueryBTCValidatorDelegationsResponse{
			BtcDelegatorDelegations: append(expiredBTCDelsList, slashableBTCDelsList...),
			Pagination:              &query.PageResponse{NextKey: nil},
		}
		mockBabylonQuerier.EXPECT().BTCValidatorDelegations(gomock.Eq(valBTCPK.MarshalHex()), gomock.Any()).Return(btcDelsResp, nil).Times(1)

		// ensure there should be only len(slashableBTCDelsList) BTC txs
		mockBTCClient.EXPECT().
			SendRawTransaction(gomock.Any(), gomock.Eq(true)).
			Return(&chainhash.Hash{}, nil).
			Times(len(slashableBTCDelsList))

		err = btcSlasher.SlashBTCValidator(valBTCPK, valSK)
		require.NoError(t, err)
	})
}
