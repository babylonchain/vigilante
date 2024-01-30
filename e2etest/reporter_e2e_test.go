//go:build e2e
// +build e2e

package e2etest

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/reporter"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

var (
	longEventuallyWaitTimeOut = 2 * eventuallyWaitTimeOut
)

func (tm *TestManager) BabylonBTCChainMatchesBtc(t *testing.T) bool {
	tipHash, tipHeight, err := tm.BTCClient.GetBestBlock()
	require.NoError(t, err)
	bbnBtcLcTip, err := tm.BabylonClient.BTCHeaderChainTip()
	require.NoError(t, err)
	return uint64(tipHeight) == bbnBtcLcTip.Header.Height && tipHash.String() == bbnBtcLcTip.Header.Hash.String()
}

func (tm *TestManager) GenerateAndSubmitsNBlocksFromTip(N int) {
	var ut time.Time

	for i := 0; i < N; i++ {
		tm.MinerNode.GenerateAndSubmitBlock(nil, -1, ut)
	}
}

func (tm *TestManager) GenerateAndSubmitBlockNBlockStartingFromDepth(t *testing.T, N int, depth uint32) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	if depth == 0 {
		// depth 0 means we are starting from tip
		tm.GenerateAndSubmitsNBlocksFromTip(N)
		return
	}

	_, bestHeight, err := tm.MinerNode.Client.GetBestBlock()
	require.NoError(t, err)

	startingBlockHeight := bestHeight - int32(depth)

	blockHash, err := tm.MinerNode.Client.GetBlockHash(int64(startingBlockHeight))
	require.NoError(t, err)

	startingBlockMsg, err := tm.MinerNode.Client.GetBlock(blockHash)
	require.NoError(t, err)

	startingBlock := btcutil.NewBlock(startingBlockMsg)
	startingBlock.SetHeight(startingBlockHeight)

	arr := datagen.GenRandomByteArray(r, 20)
	add, err := btcutil.NewAddressScriptHashFromHash(arr, tm.MinerNode.ActiveNet)
	require.NoError(t, err)

	var lastSubmittedBlock *btcutil.Block
	var ut time.Time

	for i := 0; i < N; i++ {
		var blockToSubmit *btcutil.Block

		if lastSubmittedBlock == nil {
			// first block to submit start from starting block
			newBlock, err := rpctest.CreateBlock(startingBlock, nil, rpctest.BlockVersion,
				ut, add, nil, tm.MinerNode.ActiveNet)
			require.NoError(t, err)
			blockToSubmit = newBlock
		} else {
			newBlock, err := rpctest.CreateBlock(lastSubmittedBlock, nil, rpctest.BlockVersion,
				ut, add, nil, tm.MinerNode.ActiveNet)
			require.NoError(t, err)
			blockToSubmit = newBlock
		}
		err = tm.MinerNode.Client.SubmitBlock(blockToSubmit, nil)
		require.NoError(t, err)
		lastSubmittedBlock = blockToSubmit
	}
}

func TestReporter_BoostrapUnderFrequentBTCHeaders(t *testing.T) {
	// no need to much mature outputs, we are not going to submit transactions in this test
	numMatureOutputs := uint32(2)

	blockEventChan := make(chan *types.BlockEvent, 1000)
	handlers := &rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			blockEventChan <- types.NewBlockEvent(types.BlockConnected, height, header)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			blockEventChan <- types.NewBlockEvent(types.BlockDisconnected, height, header)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, blockEventChan)
	defer tm.Stop(t)

	reporterMetrics := metrics.NewReporterMetrics()
	vigilantReporter, err := reporter.New(
		&tm.Config.Reporter,
		logger,
		tm.BTCClient,
		tm.BabylonClient,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		reporterMetrics,
	)
	require.NoError(t, err)

	// start a routine that mines BTC blocks very fast
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			tm.GenerateAndSubmitsNBlocksFromTip(1)
		}
	}()

	time.Sleep(20 * time.Second)

	// start reporter
	vigilantReporter.Start()
	defer vigilantReporter.Stop()

	// tips should eventually match
	require.Eventually(t, func() bool {
		return tm.BabylonBTCChainMatchesBtc(t)
	}, longEventuallyWaitTimeOut, eventuallyPollTime)
}

func TestRelayHeadersAndHandleRollbacks(t *testing.T) {
	// no need to much mature outputs, we are not going to submit transactions in this test
	numMatureOutputs := uint32(2)

	blockEventChan := make(chan *types.BlockEvent, 1000)
	handlers := &rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			blockEventChan <- types.NewBlockEvent(types.BlockConnected, height, header)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			blockEventChan <- types.NewBlockEvent(types.BlockDisconnected, height, header)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, blockEventChan)
	// this is necessary to receive notifications about new transactions entering mempool
	defer tm.Stop(t)

	reporterMetrics := metrics.NewReporterMetrics()

	vigilantReporter, err := reporter.New(
		&tm.Config.Reporter,
		logger,
		tm.BTCClient,
		tm.BabylonClient,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		reporterMetrics,
	)
	require.NoError(t, err)
	vigilantReporter.Start()
	defer vigilantReporter.Stop()

	require.Eventually(t, func() bool {
		return tm.BabylonBTCChainMatchesBtc(t)
	}, longEventuallyWaitTimeOut, eventuallyPollTime)

	// generate 3, we are submitting headers 1 by 1 so we use small amount as this is slow process
	tm.GenerateAndSubmitsNBlocksFromTip(3)

	require.Eventually(t, func() bool {
		return tm.BabylonBTCChainMatchesBtc(t)
	}, longEventuallyWaitTimeOut, eventuallyPollTime)

	// we will start from block before tip and submit 2 new block this should trigger rollback
	tm.GenerateAndSubmitBlockNBlockStartingFromDepth(t, 2, 1)

	// tips should eventually match
	require.Eventually(t, func() bool {
		return tm.BabylonBTCChainMatchesBtc(t)
	}, longEventuallyWaitTimeOut, eventuallyPollTime)
}

func TestHandleReorgAfterRestart(t *testing.T) {
	// no need to much mature outputs, we are not going to submit transactions in this test
	numMatureOutputs := uint32(2)

	blockEventChan := make(chan *types.BlockEvent, 1000)
	handlers := &rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			blockEventChan <- types.NewBlockEvent(types.BlockConnected, height, header)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			blockEventChan <- types.NewBlockEvent(types.BlockDisconnected, height, header)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, blockEventChan)
	// this is necessary to receive notifications about new transactions entering mempool
	defer tm.Stop(t)

	reporterMetrics := metrics.NewReporterMetrics()

	vigilantReporter, err := reporter.New(
		&tm.Config.Reporter,
		logger,
		tm.BTCClient,
		tm.BabylonClient,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		reporterMetrics,
	)
	require.NoError(t, err)

	vigilantReporter.Start()

	require.Eventually(t, func() bool {
		return tm.BabylonBTCChainMatchesBtc(t)
	}, longEventuallyWaitTimeOut, eventuallyPollTime)

	// At this point babylon is inline with btc. Now:
	// Kill reporter
	// and generate reorg on btc
	// start reporter again
	// Even though reorg happened, reporter should be able to provide better chain
	// in bootstrap phase

	vigilantReporter.Stop()
	vigilantReporter.WaitForShutdown()

	// // we will start from block before tip and submit 2 new block this should trigger rollback
	tm.GenerateAndSubmitBlockNBlockStartingFromDepth(t, 2, 1)

	// Start new reporter
	vigilantReporterNew, err := reporter.New(
		&tm.Config.Reporter,
		logger,
		tm.BTCClient,
		tm.BabylonClient,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		reporterMetrics,
	)
	require.NoError(t, err)

	vigilantReporterNew.Start()

	// Headers should match even though reorg happened
	require.Eventually(t, func() bool {
		return tm.BabylonBTCChainMatchesBtc(t)
	}, longEventuallyWaitTimeOut, eventuallyPollTime)

}
