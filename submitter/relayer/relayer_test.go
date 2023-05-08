package relayer_test

// TODO
//		 1. datagen a number of unspent txs and add them into the mock
//		 2. mock GetNetParams()
//		 3. mock GetTxFee()
//		 4. mock GetRawChangeAddress()
//		 5. mock WalletPassphrase()
//		 6. mock DumpPrivKey()
//		 7. mock SendRawTransaction()
//func FuzzSendCheckpointsToBTC(f *testing.F) {
/*
	Checks:
	- submission should fail if the checkpoint status is not Sealed
	- submission should fail if there's insufficient balance in the wallet
	- the relayer converts a raw checkpoint into BTC txs and sends them without errors
	- the content of a pair of transactions can be decoded into a checkpoint that is the same as the original one
	- submission should fail if resend the checkpoint within resendIntervals
	- resend the checkpoint without error if resendIntervals has passed

	Data generation:
	- random raw checkpoints
	- random unspent txs
*/
//datagen.AddRandomSeedsToFuzzer(f, 10)
//submitterAddr, _ := sdk.AccAddressFromBech32("bbn1eppc73j56382wjn6nnq3quu5eye4pmm087xfdh")
//f.Fuzz(func(t *testing.T, seed int64) {
//	r := rand.New(rand.NewSource(seed))
//	ckpt := datagen.GenRandomRawCheckpointWithMeta(r)
//	wallet := mocks.NewMockBTCWallet(gomock.NewController(t))
//	testRelayer := relayer.New(wallet, []byte("bbnt"), btctxformatter.CurrentVersion, submitterAddr, 10)
//	err := testRelayer.SendCheckpointToBTC(ckpt)
//	if ckpt.Status == checkpointingtypes.Sealed {
//		require.NoError(t, err)
//	} else {
//		require.Error(t, err)
//	}
//})
//}
