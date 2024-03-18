//go:build e2e
// +build e2e

package e2etest

import (
	"encoding/hex"
	"testing"
	"time"

	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/vigilante/btcclient"
	bst "github.com/babylonchain/vigilante/btcstaking-tracker"
	"github.com/babylonchain/vigilante/btcstaking-tracker/btcslasher"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func TestAtomicSlasher(t *testing.T) {
	// segwit is activated at height 300. It's needed by staking/slashing tx
	numMatureOutputs := uint32(300)

	submittedTxs := map[chainhash.Hash]struct{}{}
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
			submittedTxs[*hash] = struct{}{}
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, blockEventChan)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	err = tm.MinerNode.Client.NotifyBlocks()
	require.NoError(t, err)
	defer tm.Stop(t)
	// start WebSocket connection with Babylon for subscriber services
	err = tm.BabylonClient.Start()
	require.NoError(t, err)
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
	go bsTracker.Start()
	defer bsTracker.Stop()

	// wait for bootstrapping
	time.Sleep(5 * time.Second)

	bsParamsResp, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	bsParams := bsParamsResp.Params

	// set up a finality provider
	btcFP, fpSK := tm.CreateFinalityProvider(t)
	// set up 2 BTC delegations
	tm.CreateBTCDelegation(t, fpSK)
	tm.CreateBTCDelegation(t, fpSK)

	// retrieve 2 BTC delegations
	btcDelsResp, err := tm.BabylonClient.BTCDelegations(bstypes.BTCDelegationStatus_ACTIVE, nil)
	require.NoError(t, err)
	require.Len(t, btcDelsResp.BtcDelegations, 2)
	btcDels := btcDelsResp.BtcDelegations

	/*
		finality provider builds slashing tx witness and sends slashing tx to Bitcoin
	*/
	victimBTCDel := btcDels[0]
	victimSlashingTx, err := btcslasher.BuildSlashingTxWithWitness(victimBTCDel, &bsParams, netParams, fpSK)
	// send slashing tx to Bitcoin
	require.NoError(t, err)
	slashingTxHash, err := tm.BTCClient.SendRawTransaction(victimSlashingTx, true)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(slashingTxHash)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// mine a block that includes slashing tx, which will trigger atomic slasher
	tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{slashingTxHash}))
	// ensure slashing tx will be detected on Bitcoin
	require.Eventually(t, func() bool {
		_, ok := submittedTxs[*slashingTxHash]
		return ok
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	/*
		atomic slasher will detect the selective slashing on victim BTC delegation
		the finality provider will get slashed on Babylon
	*/
	require.Eventually(t, func() bool {
		resp, err := tm.BabylonClient.FinalityProvider(btcFP.BtcPk.MarshalHex())
		if err != nil {
			return false
		}
		return resp.FinalityProvider.SlashedBabylonHeight > 0
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	/*
		atomic slasher will slash the other BTC delegation on Bitcoin
	*/
	btcDel2 := btcDels[1]
	slashTx2, err := bstypes.NewBTCSlashingTxFromHex(btcDel2.UndelegationResponse.SlashingTxHex)
	require.NoError(t, err)
	slashingTxHash2 := slashTx2.MustGetTxHash()
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(slashingTxHash2)
		t.Logf("err of getting slashingTxHash of the BTC delegation affected by atomic slashing: %v", err)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// mine a block that includes slashing tx, which will trigger atomic slasher
	tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{slashingTxHash2}))
	// ensure slashing tx 2 will be detected on Bitcoin
	require.Eventually(t, func() bool {
		_, ok := submittedTxs[*slashingTxHash2]
		return ok
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func TestAtomicSlasher_Unbonding(t *testing.T) {
	// segwit is activated at height 300. It's needed by staking/slashing tx
	numMatureOutputs := uint32(300)

	submittedTxs := map[chainhash.Hash]struct{}{}
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
			submittedTxs[*hash] = struct{}{}
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, blockEventChan)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	err = tm.MinerNode.Client.NotifyBlocks()
	require.NoError(t, err)
	defer tm.Stop(t)
	// start WebSocket connection with Babylon for subscriber services
	err = tm.BabylonClient.Start()
	require.NoError(t, err)
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
	go bsTracker.Start()
	defer bsTracker.Stop()

	// wait for bootstrapping
	time.Sleep(5 * time.Second)

	bsParamsResp, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	bsParams := bsParamsResp.Params

	// set up a finality provider
	btcFP, fpSK := tm.CreateFinalityProvider(t)

	// set up 1st BTC delegation, which will be later used as the victim
	stakingSlashingInfo, unbondingSlashingInfo, btcDelSK := tm.CreateBTCDelegation(t, fpSK)
	btcDelsResp, err := tm.BabylonClient.BTCDelegations(bstypes.BTCDelegationStatus_ACTIVE, nil)
	require.NoError(t, err)
	require.Len(t, btcDelsResp.BtcDelegations, 1)
	victimBTCDel := btcDelsResp.BtcDelegations[0]

	// set up 2nd BTC delegation, which will be subjected to atomic slashing
	tm.CreateBTCDelegation(t, fpSK)
	btcDelsResp2, err := tm.BabylonClient.BTCDelegations(bstypes.BTCDelegationStatus_ACTIVE, nil)
	require.NoError(t, err)
	require.Len(t, btcDelsResp2.BtcDelegations, 2)
	// NOTE: `BTCDelegations` API does not return BTC delegations in created time order
	// thus we need to find out the 2nd BTC delegation one-by-one
	var btcDel2 *bstypes.BTCDelegationResponse
	for i := range btcDelsResp2.BtcDelegations {
		if btcDelsResp2.BtcDelegations[i].StakingTxHex != victimBTCDel.StakingTxHex {
			btcDel2 = btcDelsResp2.BtcDelegations[i]
			break
		}
	}

	/*
		the victim BTC delegation unbonds
	*/
	tm.Undelegate(t, stakingSlashingInfo, unbondingSlashingInfo, btcDelSK)

	/*
		finality provider builds unbonding slashing tx witness and sends it to Bitcoin
	*/
	victimUnbondingSlashingTx, err := btcslasher.BuildUnbondingSlashingTxWithWitness(victimBTCDel, &bsParams, netParams, fpSK)
	require.NoError(t, err)
	// send slashing tx to Bitcoin
	// NOTE: sometimes unbonding slashing tx is not immediately spendable for some reason
	var unbondingSlashingTxHash *chainhash.Hash
	require.Eventually(t, func() bool {
		unbondingSlashingTxHash, err = tm.BTCClient.SendRawTransaction(victimUnbondingSlashingTx, true)
		if err != nil {
			t.Logf("err of SendRawTransaction: %v", err)
			return false
		}
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// unbonding slashing tx is eventually queryable
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(unbondingSlashingTxHash)
		if err != nil {
			t.Logf("err of GetRawTransaction: %v", err)
			return false
		}
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// mine a block that includes unbonding slashing tx, which will trigger atomic slasher
	tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{unbondingSlashingTxHash}))
	// ensure unbonding slashing tx will be detected on Bitcoin
	require.Eventually(t, func() bool {
		_, ok := submittedTxs[*unbondingSlashingTxHash]
		return ok
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	/*
		atomic slasher will detect the selective slashing on victim BTC delegation
		the finality provider will get slashed on Babylon
	*/
	require.Eventually(t, func() bool {
		resp, err := tm.BabylonClient.FinalityProvider(btcFP.BtcPk.MarshalHex())
		if err != nil {
			return false
		}
		return resp.FinalityProvider.SlashedBabylonHeight > 0
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	/*
		atomic slasher will slash the other BTC delegation on Bitcoin
	*/
	slashingTx2, err := bstypes.NewBTCSlashingTxFromHex(btcDel2.UndelegationResponse.SlashingTxHex)
	require.NoError(t, err)
	slashingTxHash2 := slashingTx2.MustGetTxHash()
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(slashingTxHash2)
		t.Logf("err of getting slashingTxHash of the BTC delegation affected by atomic slashing: %v", err)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// mine a block that includes slashing tx, which will trigger atomic slasher
	tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{slashingTxHash2}))
	// ensure slashing tx 2 will be detected on Bitcoin
	require.Eventually(t, func() bool {
		_, ok := submittedTxs[*slashingTxHash2]
		return ok
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}
