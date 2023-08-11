//go:build e2e
// +build e2e

package e2etest

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/submitter"
)

func TestSubmitterSubmission(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	numMatureOutputs := uint32(5)

	var submittedTransactions []*chainhash.Hash

	// We are setting handler for transaction hitting the mempool, to be sure we will
	// pass transaction to the miner, in the same order as they were submitted by submitter
	handlers := &rpcclient.NotificationHandlers{
		OnTxAccepted: func(hash *chainhash.Hash, amount btcutil.Amount) {
			submittedTransactions = append(submittedTransactions, hash)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, nil)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	defer tm.Stop(t)

	randomCheckpoint := datagen.GenRandomRawCheckpointWithMeta(r)
	randomCheckpoint.Status = checkpointingtypes.Sealed
	randomCheckpoint.Ckpt.EpochNum = 1

	ctl := gomock.NewController(t)
	mockBabylonClient := submitter.NewMockBabylonQueryClient(ctl)
	subAddr, _ := sdk.AccAddressFromBech32(submitterAddrStr)

	mockBabylonClient.EXPECT().BTCCheckpointParams().Return(
		&btcctypes.QueryParamsResponse{
			Params: btcctypes.Params{
				CheckpointTag:                 babylonTagHex,
				BtcConfirmationDepth:          2,
				CheckpointFinalizationTimeout: 4,
			},
		}, nil)
	mockBabylonClient.EXPECT().RawCheckpointList(gomock.Any(), gomock.Any()).Return(
		&checkpointingtypes.QueryRawCheckpointListResponse{
			RawCheckpoints: []*checkpointingtypes.RawCheckpointWithMeta{
				randomCheckpoint,
			},
		}, nil).AnyTimes()

	tm.Config.Submitter.PollingIntervalSeconds = 2
	// create submitter
	vigilantSubmitter, _ := submitter.New(
		&tm.Config.Submitter,
		tm.BTCWalletClient,
		mockBabylonClient,
		subAddr,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		metrics.NewSubmitterMetrics(),
	)

	vigilantSubmitter.Start()

	defer func() {
		vigilantSubmitter.Stop()
		vigilantSubmitter.WaitForShutdown()
	}()

	// wait for our 2 op_returns with epoch 1 checkpoint to hit the mempool and then
	// retrieve them from there
	//
	// TODO: to assert that those are really transactions send by submitter, we would
	// need to expose sentCheckpointInfo from submitter
	require.Eventually(t, func() bool {
		return len(submittedTransactions) == 2
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	sendTransactions := tm.RetrieveTransactionFromMempool(t, submittedTransactions)
	// mine a block with those transactions
	blockWithOpReturnTranssactions := tm.MineBlockWithTxs(t, sendTransactions)
	// block should have 3 transactions, 2 from submitter and 1 coinbase
	require.Equal(t, len(blockWithOpReturnTranssactions.Transactions), 3)
}

func TestSubmitterSubmissionReplace(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	numMatureOutputs := uint32(5)

	var submittedTransactions []*chainhash.Hash

	// We are setting handler for transaction hitting the mempool, to be sure we will
	// pass transaction to the miner, in the same order as they were submitted by submitter
	handlers := &rpcclient.NotificationHandlers{
		OnTxAccepted: func(hash *chainhash.Hash, amount btcutil.Amount) {
			submittedTransactions = append(submittedTransactions, hash)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, nil)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	defer tm.Stop(t)

	randomCheckpoint := datagen.GenRandomRawCheckpointWithMeta(r)
	randomCheckpoint.Status = checkpointingtypes.Sealed
	randomCheckpoint.Ckpt.EpochNum = 1

	ctl := gomock.NewController(t)
	mockBabylonClient := submitter.NewMockBabylonQueryClient(ctl)
	subAddr, _ := sdk.AccAddressFromBech32(submitterAddrStr)

	mockBabylonClient.EXPECT().BTCCheckpointParams().Return(
		&btcctypes.QueryParamsResponse{
			Params: btcctypes.Params{
				CheckpointTag:                 babylonTagHex,
				BtcConfirmationDepth:          2,
				CheckpointFinalizationTimeout: 4,
			},
		}, nil)
	mockBabylonClient.EXPECT().RawCheckpointList(gomock.Any(), gomock.Any()).Return(
		&checkpointingtypes.QueryRawCheckpointListResponse{
			RawCheckpoints: []*checkpointingtypes.RawCheckpointWithMeta{
				randomCheckpoint,
			},
		}, nil).AnyTimes()

	tm.Config.Submitter.PollingIntervalSeconds = 2
	tm.Config.Submitter.ResendIntervalSeconds = 2
	tm.Config.Submitter.ResubmitFeeMultiplier = 2.1
	// create submitter
	vigilantSubmitter, _ := submitter.New(
		&tm.Config.Submitter,
		tm.BTCWalletClient,
		mockBabylonClient,
		subAddr,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		metrics.NewSubmitterMetrics(),
	)

	vigilantSubmitter.Start()

	defer func() {
		vigilantSubmitter.Stop()
		vigilantSubmitter.WaitForShutdown()
	}()

	// wait for our 2 op_returns with epoch 1 checkpoint to hit the mempool and then
	// retrieve them from there
	//
	// TODO: to assert that those are really transactions send by submitter, we would
	// need to expose sentCheckpointInfo from submitter
	require.Eventually(t, func() bool {
		return len(submittedTransactions) == 2
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	sendTransactions := tm.RetrieveTransactionFromMempool(t, submittedTransactions)

	// at this point our submitter already sent 2 checkpoint transactions which landed in mempool.
	// Zero out submittedTransactions, and wait for a new tx2 to be submitted and accepted
	// it should be replacements for the previous one.
	submittedTransactions = []*chainhash.Hash{}

	require.Eventually(t, func() bool {
		// we only replace tx2 of the checkpoint, thus waiting for 1 tx to arrive
		return len(submittedTransactions) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	transactionReplacement := tm.RetrieveTransactionFromMempool(t, submittedTransactions)
	resendTx2 := transactionReplacement[0]

	// Here check that sendTransactions1 are replacements for sendTransactions, i.e they should have:
	// 1. same
	// 2. outputs with different values
	// 3. different signatures
	require.Equal(t, sendTransactions[1].MsgTx().TxIn[0].PreviousOutPoint, resendTx2.MsgTx().TxIn[0].PreviousOutPoint)
	require.Less(t, resendTx2.MsgTx().TxOut[1].Value, sendTransactions[1].MsgTx().TxOut[1].Value)
	require.NotEqual(t, sendTransactions[1].MsgTx().TxIn[0].SignatureScript, resendTx2.MsgTx().TxIn[0].SignatureScript)

	// mine a block with those replacement transactions just to be sure they execute correctly
	sendTransactions[1] = resendTx2
	blockWithOpReturnTransactions := tm.MineBlockWithTxs(t, sendTransactions)
	// block should have 2 transactions, 1 from submitter and 1 coinbase
	require.Equal(t, len(blockWithOpReturnTransactions.Transactions), 3)
}
