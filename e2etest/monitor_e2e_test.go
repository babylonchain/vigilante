//go:build e2e
// +build e2e

package e2etest

import (
	"testing"
	"time"

	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/stretchr/testify/require"
)

func TestMonitor_GracefulShutdown(t *testing.T) {
	numMatureOutputs := uint32(5)

	var submittedTxs []*chainhash.Hash

	// We are setting handler for transaction hitting the mempool, to be sure we will
	// pass transaction to the miner, in the same order as they were submitted by submitter
	handlers := &rpcclient.NotificationHandlers{
		OnTxAccepted: func(hash *chainhash.Hash, amount btcutil.Amount) {
			submittedTxs = append(submittedTxs, hash)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	defer tm.Stop(t)

	// create monitor
	genesisInfo := tm.getGenesisInfo(t)
	monitorMetrics := metrics.NewMonitorMetrics()
	tm.Config.Monitor.EnableLivenessChecker = false // we don't test liveness checker in this test case
	vigilanteMonitor, err := monitor.New(
		&tm.Config.Monitor,
		genesisInfo,
		tm.BabylonClient.QueryClient,
		tm.BTCClient,
		monitorMetrics,
	)
	// start monitor
	go vigilanteMonitor.Start()
	// wait for bootstrapping
	time.Sleep(5 * time.Second)
	// gracefully shut down
	defer vigilanteMonitor.Stop()
}

func (tm *TestManager) getGenesisInfo(t *testing.T) *types.GenesisInfo {
	// base BTC height
	baseHeaderResp, err := tm.BabylonClient.BTCBaseHeader()
	require.NoError(t, err)
	baseBTCHeight := baseHeaderResp.Header.Height
	// epoch interval
	epochIntervalResp, err := tm.BabylonClient.EpochingParams()
	require.NoError(t, err)
	epochInterval := epochIntervalResp.Params.EpochInterval
	// checkpoint tag
	checkpointTagResp, err := tm.BabylonClient.BTCCheckpointParams()
	require.NoError(t, err)
	checkpointTag := checkpointTagResp.Params.CheckpointTag
	// val set
	valSetResp, err := tm.BabylonClient.BlsPublicKeyList(0, nil)
	require.NoError(t, err)
	valSet := &checkpointingtypes.ValidatorWithBlsKeySet{
		ValSet: valSetResp.ValidatorWithBlsKeys,
	}
	return types.NewGenesisInfo(baseBTCHeight, epochInterval, checkpointTag, valSet)
}
