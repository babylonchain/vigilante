//go:build e2e
// +build e2e

package e2etest

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/babylonchain/babylon/btcstaking"
	"github.com/babylonchain/vigilante/btcclient"
	bst "github.com/babylonchain/vigilante/btcstaking-tracker"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func TestUnbondingWatcher(t *testing.T) {
	// segwit is activated at height 300. It's needed by staking/slashing tx
	numMatureOutputs := uint32(300)

	submittedTxs := []*chainhash.Hash{}
	blockEventChan := make(chan *types.BlockEvent, 1000)
	handlers := &rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			log.Debugf("Block %v at height %d has been connected at time %v", header.BlockHash(), height, header.Timestamp)
			blockEventChan <- types.NewBlockEvent(types.BlockConnected, height, header)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			log.Debugf("Block %v at height %d has been disconnected at time %v", header.BlockHash(), height, header.Timestamp)
			blockEventChan <- types.NewBlockEvent(types.BlockDisconnected, height, header)
		},
		OnTxAccepted: func(hash *chainhash.Hash, amount btcutil.Amount) {
			submittedTxs = append(submittedTxs, hash)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, blockEventChan)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	err = tm.MinerNode.Client.NotifyBlocks()
	require.NoError(t, err)
	defer tm.Stop(t)
	// Insert all existing BTC headers to babylon node
	tm.CatchUpBTCLightClient(t)

	emptyHintCache := btcclient.EmptyHintCache{}

	// TODO: our config only support btcd wallet tls, not btcd directly
	tm.Config.BTC.DisableClientTLS = false
	backend, err := btcclient.NewNodeBackend(
		btcclient.CfgToBtcNodeBackendConfig(tm.Config.BTC, hex.EncodeToString(tm.MinerNode.RPCConfig().Certificates)),
		&chaincfg.SimNetParams,
		&emptyHintCache,
	)
	require.NoError(t, err)

	err = backend.Start()
	require.NoError(t, err)

	commonCfg := config.DefaultCommonConfig()
	bstCfg := config.DefaultBTCStakingTrackerConfig()
	bstCfg.CheckDelegationsInterval = 1 * time.Second
	logger, err := config.NewRootLogger("auto", "debug")
	require.NoError(t, err)

	metrics := metrics.NewBTCStakingTrackerMetrics()

	bsTracker := bst.NewBTCSTakingTracker(
		tm.BTCClient,
		backend,
		tm.BabylonClient,
		&bstCfg,
		&commonCfg,
		logger,
		metrics,
	)
	bsTracker.Start()
	defer bsTracker.Stop()

	// set up a finality provider
	_, fpSK := tm.CreateFinalityProvider(t)
	logger.Info("created finality provider")
	// set up a BTC delegation
	stakingSlashingInfo, unbondingSlashingInfo, delSK := tm.CreateBTCDelegation(t, fpSK)
	logger.Info("created BTC delegation")

	// Staker unbonds by directly sending tx to btc network. Watcher should detect it and report to babylon.
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	stakingOutIdx, err := outIdx(unbondingSlashingInfo.UnbondingTx, unbondingSlashingInfo.UnbondingInfo.UnbondingOutput)
	require.NoError(t, err)
	unbondingTxSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingSlashingInfo.UnbondingTx,
		stakingSlashingInfo.StakingTx,
		stakingOutIdx,
		unbondingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)
	resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
	require.NoError(t, err)
	covenantSigs := resp.BtcDelegation.UndelegationResponse.CovenantUnbondingSigList
	witness, err := unbondingPathSpendInfo.CreateUnbondingPathWitness(
		[]*schnorr.Signature{covenantSigs[0].Sig.MustToBTCSig()},
		unbondingTxSchnorrSig,
	)
	unbondingSlashingInfo.UnbondingTx.TxIn[0].Witness = witness
	// Send unbonding tx to Bitcoin
	_, err = tm.BTCWalletClient.SendRawTransaction(unbondingSlashingInfo.UnbondingTx, true)
	require.NoError(t, err)
	// mine a block with this tx, and insert it to Bitcoin
	unbondingTxHash := unbondingSlashingInfo.UnbondingTx.TxHash()
	t.Logf("submitted unbonding tx with hash %s", unbondingTxHash.String())
	mBlock := tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{&unbondingTxHash}))
	require.Equal(t, 2, len(mBlock.Transactions))

	require.Eventually(t, func() bool {
		resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
		require.NoError(t, err)

		// TODO: Add field for staker signature in BTCDelegation query to check it directly,
		// for now it is enough to check that delegation is not active, as if unbonding was reported
		// delegation will be deactivated
		return !resp.BtcDelegation.Active

	}, eventuallyWaitTimeOut, eventuallyPollTime)
}
