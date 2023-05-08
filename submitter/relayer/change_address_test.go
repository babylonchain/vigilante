package relayer_test

import (
	"testing"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/txscript"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/submitter/relayer"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/babylonchain/vigilante/types"
)

var submitterAddrStr = "bbn1eppc73j56382wjn6nnq3quu5eye4pmm087xfdh"

// obtained from https://secretscan.org/Bech32
var SegWitBech32p2wpkhAddrsStr = []string{
	"bc1qdh5ezhcx5fh7mlk0qwmy0pw89pxklnmrd9nwwr",
	"bc1qyujvyayzdr2znhsepa2cw40w7pkz0afr8hkxsg",
}

var SegWitBech32p2wshAddrsStr = []string{
	"bc1q7gytzww8cnzgp390q8ztvkfl5r3pmzh3y7dxuvqwvd52ceq7034qldnk59",
	"bc1q6fceckkklar0qtx8w66x60qrafalruu5upllx8f0jdanwz8gex4sx79eml",
}

var legacyAddrsStr = []string{
	"1GApPLw7MZsgvDrKKSi2GyN3uepup8w9ib",
	"1MzfDjLv3qwRyEJkF7kgviJnqVhH8och6N",
}

func TestGetChangeAddress(t *testing.T) {
	submitterAddr, err := sdk.AccAddressFromBech32(submitterAddrStr)
	require.NoError(t, err)
	wallet := mocks.NewMockBTCWallet(gomock.NewController(t))
	wallet.EXPECT().GetNetParams().Return(netparams.GetBTCParams(types.BtcMainnet.String())).AnyTimes()
	testRelayer := relayer.New(wallet, []byte("bbnt"), btctxformatter.CurrentVersion, submitterAddr, 10)

	// 1. only SegWit Bech32 addresses
	segWitBech32Addrs := append(SegWitBech32p2wshAddrsStr, SegWitBech32p2wpkhAddrsStr...)
	wallet.EXPECT().ListUnspent().Return(getAddrsResult(segWitBech32Addrs), nil)
	changeAddr, err := testRelayer.GetChangeAddress()
	require.NoError(t, err)
	require.True(t, contains(segWitBech32Addrs, changeAddr.String()))
	_, err = txscript.PayToAddrScript(changeAddr)
	require.NoError(t, err)

	// 2. only legacy addresses
	wallet.EXPECT().ListUnspent().Return(getAddrsResult(legacyAddrsStr), nil)
	changeAddr, err = testRelayer.GetChangeAddress()
	require.NoError(t, err)
	require.True(t, contains(legacyAddrsStr, changeAddr.String()))
	_, err = txscript.PayToAddrScript(changeAddr)
	require.NoError(t, err)

	// 3. SegWit-Bech32 + legacy addresses, should only return SegWit-Bech32 addresses
	addrs := append(segWitBech32Addrs, legacyAddrsStr...)
	wallet.EXPECT().ListUnspent().Return(getAddrsResult(addrs), nil)
	changeAddr, err = testRelayer.GetChangeAddress()
	require.NoError(t, err)
	require.True(t, contains(segWitBech32Addrs, changeAddr.String()))
	_, err = txscript.PayToAddrScript(changeAddr)
	require.NoError(t, err)
}

func getAddrsResult(addressesStr []string) []btcjson.ListUnspentResult {
	var addrsRes []btcjson.ListUnspentResult
	for _, addrStr := range addressesStr {
		res := btcjson.ListUnspentResult{Address: addrStr}
		addrsRes = append(addrsRes, res)
	}

	return addrsRes
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
