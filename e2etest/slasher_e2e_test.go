//go:build e2e
// +build e2e

package e2etest

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/btcstaking"
	"github.com/babylonchain/babylon/crypto/eots"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/btcclient"
	bst "github.com/babylonchain/vigilante/btcstaking-tracker"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))
	// finality provider
	fpSK, _, _    = datagen.GenRandomBTCKeyPair(r)
	btcFp, _      = datagen.GenRandomFinalityProviderWithBTCSK(r, fpSK)
	btcfpBTCPK, _ = btcFp.BtcPk.ToBTCPK()
	// BTC delegation
	delBabylonSK, delBabylonPK, _ = datagen.GenRandomSecp256k1KeyPair(r)

	// covenant
	covenantSk, _ = btcec.PrivKeyFromBytes(
		[]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	)
	// change address
	changeAddress, _ = datagen.GenRandomBTCAddress(r, netParams)
)

func TestSlasher_GracefulShutdown(t *testing.T) {
	numMatureOutputs := uint32(5)

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

	// TODO:L our config only support btcd wallet tls, not btcd dierectly
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

	// wait for bootstrapping
	time.Sleep(10 * time.Second)

	// gracefully shut down
	defer bsTracker.Stop()
}

func TestSlasher_Slasher(t *testing.T) {
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
	// start WebSocket connection with Babylon for subscriber services
	err = tm.BabylonClient.Start()
	require.NoError(t, err)
	// Insert all existing BTC headers to babylon node
	tm.CatchUpBTCLightClient(t)

	emptyHintCache := btcclient.EmptyHintCache{}

	// TODO:L our config only support btcd wallet tls, not btcd dierectly
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

	// set up a finality provider
	tm.createFinalityProvider(t)
	// set up a BTC delegation
	stakingSlashingInfo, _, _ := tm.createBTCDelegation(t)

	// commit public randomness, vote and equivocate
	tm.voteAndEquivocate(t)

	// slashing tx will eventually enter mempool
	slashingMsgTx, err := stakingSlashingInfo.SlashingTx.ToMsgTx()
	require.NoError(t, err)
	slashingMsgTxHash1 := slashingMsgTx.TxHash()
	slashingMsgTxHash := &slashingMsgTxHash1
	// slashing tx will eventually enter mempool
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(slashingMsgTxHash)
		t.Logf("err of getting slashingMsgTxHash: %v", err)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// mine a block that includes slashing tx
	tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{slashingMsgTxHash}))
	// ensure 2 txs will eventually be received (staking tx and slashing tx)
	require.Eventually(t, func() bool {
		return len(submittedTxs) == 2
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func TestSlasher_SlashingUnbonding(t *testing.T) {
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
	// start WebSocket connection with Babylon for subscriber services
	err = tm.BabylonClient.Start()
	require.NoError(t, err)
	// Insert all existing BTC headers to babylon node
	tm.CatchUpBTCLightClient(t)

	emptyHintCache := btcclient.EmptyHintCache{}

	// TODO:L our config only support btcd wallet tls, not btcd dierectly
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

	// set up a finality provider
	tm.createFinalityProvider(t)
	// set up a BTC delegation
	_, _, _ = tm.createBTCDelegation(t)
	// set up a BTC delegation
	stakingSlashingInfo1, unbondingSlashingInfo1, stakerPrivKey1 := tm.createBTCDelegation(t)

	// undelegate
	unbondingSlashingInfo, _ := tm.undelegate(t, stakingSlashingInfo1, unbondingSlashingInfo1, stakerPrivKey1)

	// commit public randomness, vote and equivocate
	tm.voteAndEquivocate(t)

	// slashing tx will eventually enter mempool
	unbondingSlashingMsgTx, err := unbondingSlashingInfo.SlashingTx.ToMsgTx()
	require.NoError(t, err)
	unbondingSlashingMsgTxHash1 := unbondingSlashingMsgTx.TxHash()
	unbondingSlashingMsgTxHash := &unbondingSlashingMsgTxHash1

	// slash unbonding tx will eventually enter mempool
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(unbondingSlashingMsgTxHash)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// mine a block that includes slashing tx
	tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{unbondingSlashingMsgTxHash}))
	// ensure tx is eventually on Bitcoin
	require.Eventually(t, func() bool {
		res, err := tm.BTCClient.GetRawTransactionVerbose(unbondingSlashingMsgTxHash)
		if err != nil {
			return false
		}
		return len(res.BlockHash) > 0
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func TestSlasher_Bootstrapping(t *testing.T) {
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
	// start WebSocket connection with Babylon for subscriber services
	err = tm.BabylonClient.Start()
	require.NoError(t, err)
	// Insert all existing BTC headers to babylon node
	tm.CatchUpBTCLightClient(t)

	// set up a finality provider
	tm.createFinalityProvider(t)
	// set up a BTC delegation
	stakingSlashingInfo, _, _ := tm.createBTCDelegation(t)

	// commit public randomness, vote and equivocate
	tm.voteAndEquivocate(t)

	emptyHintCache := btcclient.EmptyHintCache{}
	// TODO:L our config only support btcd wallet tls, not btcd dierectly
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

	// bootstrap BTC staking tracker
	err = bsTracker.Bootstrap(0)
	require.NoError(t, err)

	// slashing tx will eventually enter mempool
	slashingMsgTx, err := stakingSlashingInfo.SlashingTx.ToMsgTx()
	require.NoError(t, err)
	slashingMsgTxHash1 := slashingMsgTx.TxHash()
	slashingMsgTxHash := &slashingMsgTxHash1
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(slashingMsgTxHash)
		t.Logf("err of getting slashingMsgTxHash: %v", err)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// mine a block that includes slashing tx
	tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{slashingMsgTxHash}))
	// ensure 2 txs will eventually be received (staking tx and slashing tx)
	require.Eventually(t, func() bool {
		return len(submittedTxs) == 2
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func (tm *TestManager) createFinalityProvider(t *testing.T) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	/*
		create finality provider
	*/
	commission := sdkmath.LegacyZeroDec()
	msgNewVal := &bstypes.MsgCreateFinalityProvider{
		Signer:      signerAddr,
		Description: &stakingtypes.Description{Moniker: datagen.GenRandomHexStr(r, 10)},
		Commission:  &commission,
		BabylonPk:   btcFp.BabylonPk,
		BtcPk:       btcFp.BtcPk,
		Pop:         btcFp.Pop,
	}
	_, err := tm.BabylonClient.ReliablySendMsg(context.Background(), msgNewVal, nil, nil)
	require.NoError(t, err)
}

func (tm *TestManager) createBTCDelegation(
	t *testing.T,
) (*datagen.TestStakingSlashingInfo, *datagen.TestUnbondingSlashingInfo, *btcec.PrivateKey) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	/*
		create BTC delegation
	*/
	// generate staking tx and slashing tx
	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	covenantBtcPks, err := bbnPksToBtcPks(bsParams.Params.CovenantPks)
	require.NoError(t, err)
	stakingTimeBlocks := uint16(math.MaxUint16)
	// get top UTXO
	topUnspentResult, _, err := tm.BTCWalletClient.GetHighUTXOAndSum()
	require.NoError(t, err)
	topUTXO, err := types.NewUTXO(topUnspentResult, netParams)
	// staking value
	stakingValue := int64(topUTXO.Amount) / 3
	// dump SK
	wif, err := tm.BTCWalletClient.DumpPrivKey(topUTXO.Addr)
	require.NoError(t, err)

	// generate legitimate BTC del
	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfoWithOutPoint(
		r,
		t,
		netParams,
		topUTXO.GetOutPoint(),
		wif.PrivKey,
		[]*btcec.PublicKey{btcfpBTCPK},
		covenantBtcPks,
		bsParams.Params.CovenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		bsParams.Params.SlashingAddress,
		changeAddress.String(),
		bsParams.Params.SlashingRate,
	)
	// sign staking tx and overwrite the staking tx to the signed version
	// NOTE: the tx hash has changed here since stakingMsgTx is pre-segwit
	stakingMsgTx, signed, err := tm.BTCWalletClient.SignRawTransaction(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)
	require.True(t, signed)
	// overwrite staking tx
	stakingSlashingInfo.StakingTx = stakingMsgTx
	// get signed staking tx hash
	stakingMsgTxHash1 := stakingSlashingInfo.StakingTx.TxHash()
	stakingMsgTxHash := &stakingMsgTxHash1
	t.Logf("signed staking tx hash: %s", stakingMsgTxHash.String())

	// change outpoint tx hash of slashing tx to the txhash of the signed staking tx
	slashingMsgTx, err := stakingSlashingInfo.SlashingTx.ToMsgTx()
	require.NoError(t, err)
	slashingMsgTx.TxIn[0].PreviousOutPoint.Hash = stakingSlashingInfo.StakingTx.TxHash()
	// update slashing tx
	stakingSlashingInfo.SlashingTx, err = bstypes.NewBTCSlashingTxFromMsgTx(slashingMsgTx)
	require.NoError(t, err)

	// send staking tx to Bitcoin node's mempool
	_, err = tm.BTCWalletClient.SendRawTransaction(stakingMsgTx, true)
	require.NoError(t, err)

	// mine a block with this tx, and insert it to Bitcoin / Babylon
	mBlock := tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{stakingMsgTxHash}))
	require.Equal(t, 2, len(mBlock.Transactions))
	// get spv proof of the BTC staking tx
	stakingTxInfo := getTxInfo(t, mBlock, 1)

	// insert k empty blocks to Bitcoin
	btccParamsResp, err := tm.BabylonClient.BTCCheckpointParams()
	require.NoError(t, err)
	btccParams := btccParamsResp.Params
	for i := 0; i < int(btccParams.BtcConfirmationDepth); i++ {
		tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{}))
	}

	stakingOutIdx, err := outIdx(stakingSlashingInfo.StakingTx, stakingSlashingInfo.StakingInfo.StakingOutput)
	require.NoError(t, err)
	// create PoP
	pop, err := bstypes.NewPoP(delBabylonSK, wif.PrivKey)
	require.NoError(t, err)
	slashingSpendPath, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	// generate proper delegator sig
	require.NoError(t, err)
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		wif.PrivKey,
	)
	require.NoError(t, err)

	// Genearate all data necessary for unbonding
	fee := int64(1000)
	unbodingTimeBlocks := uint16(100)
	unbondingValue := stakingSlashingInfo.StakingInfo.StakingOutput.Value - fee
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		netParams,
		wif.PrivKey,
		[]*btcec.PublicKey{btcfpBTCPK},
		covenantBtcPks,
		bsParams.Params.CovenantQuorum,
		wire.NewOutPoint(stakingMsgTxHash, stakingOutIdx),
		unbodingTimeBlocks,
		unbondingValue,
		bsParams.Params.SlashingAddress,
		changeAddress.String(),
		bsParams.Params.SlashingRate,
	)
	require.NoError(t, err)
	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)

	unbondingSlashingPathSpendInfo, err := unbondingSlashingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	slashingTxSig, err := unbondingSlashingInfo.SlashingTx.Sign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding tx
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		wif.PrivKey,
	)
	require.NoError(t, err)

	// 	Build message to send
	tm.CatchUpBTCLightClient(t)

	// submit BTC delegation to Babylon
	msgBTCDel := &bstypes.MsgCreateBTCDelegation{
		Signer:               signerAddr,
		BabylonPk:            delBabylonPK.(*secp256k1.PubKey),
		Pop:                  pop,
		BtcPk:                bbn.NewBIP340PubKeyFromBTCPK(wif.PrivKey.PubKey()),
		FpBtcPkList:          []bbn.BIP340PubKey{*btcFp.BtcPk},
		StakingTime:          uint32(stakingTimeBlocks),
		StakingValue:         stakingValue,
		StakingTx:            stakingTxInfo,
		SlashingTx:           stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig: delegatorSig,
		// Ubonding related data
		UnbondingTx:                   unbondingTxBytes,
		UnbondingTime:                 uint32(unbodingTimeBlocks),
		UnbondingValue:                unbondingSlashingInfo.UnbondingInfo.UnbondingOutput.Value,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: slashingTxSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgBTCDel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgCreateBTCDelegation")

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// TODO: Make this handle multiple covenant signatures
	fpEncKey, err := asig.NewEncryptionKeyFromBTCPK(fpSK.PubKey())
	require.NoError(t, err)
	covenantSig, err := stakingSlashingInfo.SlashingTx.EncSign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		covenantSk,
		fpEncKey,
	)
	require.NoError(t, err)

	// TODO: Add covenant sigs for all covenants
	// add covenant sigs
	// covenant Schnorr sig on unbonding tx
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	unbondingTxCovenantSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingSlashingInfo.UnbondingTx,
		stakingSlashingInfo.StakingTx,
		stakingOutIdx,
		unbondingPathSpendInfo.GetPkScriptPath(),
		covenantSk,
	)
	require.NoError(t, err)
	covenantUnbondingSig := bbn.NewBIP340SignatureFromBTCSig(unbondingTxCovenantSchnorrSig)
	// covenant adaptor sig on unbonding slashing tx
	require.NoError(t, err)
	covenantSlashingSig, err := unbondingSlashingInfo.SlashingTx.EncSign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding transaction
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		covenantSk,
		fpEncKey,
	)
	require.NoError(t, err)
	msgAddCovenantSig := &bstypes.MsgAddCovenantSigs{
		Signer:                  signerAddr,
		Pk:                      bbn.NewBIP340PubKeyFromBTCPK(covenantSk.PubKey()),
		StakingTxHash:           stakingSlashingInfo.StakingTx.TxHash().String(),
		SlashingTxSigs:          [][]byte{covenantSig.MustMarshal()},
		UnbondingTxSig:          covenantUnbondingSig,
		SlashingUnbondingTxSigs: [][]byte{covenantSlashingSig.MustMarshal()},
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgAddCovenantSig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted covenant signature")
	return stakingSlashingInfo, unbondingSlashingInfo, wif.PrivKey
}

func (tm *TestManager) undelegate(
	t *testing.T,
	stakingSlashingInfo *datagen.TestStakingSlashingInfo,
	unbondingSlashingInfo *datagen.TestUnbondingSlashingInfo,
	delSK *btcec.PrivateKey) (*datagen.TestUnbondingSlashingInfo, *schnorr.Signature) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	// TODO: This generates unbonding tx signature, move it to undelegate
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	// the only input to unbonding tx is the staking tx
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

	msgUndel := &bstypes.MsgBTCUndelegate{
		Signer:         signerAddr,
		StakingTxHash:  stakingSlashingInfo.StakingTx.TxHash().String(),
		UnbondingTxSig: bbn.NewBIP340SignatureFromBTCSig(unbondingTxSchnorrSig),
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgUndel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgBTCUndelegate")

	// TODO: use multiple covenants
	resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
	require.NoError(t, err)
	covenantSigs := resp.UndelegationInfo.CovenantUnbondingSigList
	witness, err := unbondingPathSpendInfo.CreateUnbondingPathWitness(
		[]*schnorr.Signature{covenantSigs[0].Sig.MustToBTCSig()},
		unbondingTxSchnorrSig,
	)
	unbondingSlashingInfo.UnbondingTx.TxIn[0].Witness = witness

	// send unbonding tx to Bitcoin node's mempool
	_, err = tm.BTCWalletClient.SendRawTransaction(unbondingSlashingInfo.UnbondingTx, true)
	require.NoError(t, err)
	// mine a block with this tx, and insert it to Bitcoin
	unbondingTxHash := unbondingSlashingInfo.UnbondingTx.TxHash()
	t.Logf("submitted unbonding tx with hash %s", unbondingTxHash.String())
	mBlock := tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{&unbondingTxHash}))
	require.Equal(t, 2, len(mBlock.Transactions))
	return unbondingSlashingInfo, unbondingTxSchnorrSig
}

func (tm *TestManager) voteAndEquivocate(t *testing.T) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	/*
		commit a number of public randomness since activatedHeight
	*/
	// commit public randomness list
	require.Eventually(t, func() bool {
		// need to wait for activatedHeight to not return error
		_, err := tm.BabylonClient.ActivatedHeight()
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	activatedHeightResp, err := tm.BabylonClient.ActivatedHeight()
	require.NoError(t, err)
	activatedHeight := activatedHeightResp.Height
	srList, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, fpSK, activatedHeight, 100)
	require.NoError(t, err)
	msgCommitPubRandList.Signer = signerAddr
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgCommitPubRandList, nil, nil)
	require.NoError(t, err)
	t.Logf("committed public randomness")

	/*
		submit finality signature
	*/
	// get block to vote
	blockToVote, err := tm.BabylonClient.GetBlock(int64(activatedHeight))
	require.NoError(t, err)
	msgToSign := append(sdk.Uint64ToBigEndian(activatedHeight), blockToVote.Block.AppHash...)
	// generate EOTS signature
	sig, err := eots.Sign(fpSK, srList[0], msgToSign)
	require.NoError(t, err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)
	// submit finality signature
	msgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		FpBtcPk:      btcFp.BtcPk,
		BlockHeight:  activatedHeight,
		BlockAppHash: blockToVote.Block.AppHash,
		FinalitySig:  eotsSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgAddFinalitySig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted finality signature")

	/*
		equivocate
	*/
	invalidAppHash := datagen.GenRandomByteArray(r, 32)
	invalidMsgToSign := append(sdk.Uint64ToBigEndian(activatedHeight), invalidAppHash...)
	invalidSig, err := eots.Sign(fpSK, srList[0], invalidMsgToSign)
	require.NoError(t, err)
	invalidEotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(invalidSig)
	invalidMsgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		FpBtcPk:      btcFp.BtcPk,
		BlockHeight:  activatedHeight,
		BlockAppHash: invalidAppHash,
		FinalitySig:  invalidEotsSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), invalidMsgAddFinalitySig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted equivocating finality signature")
}

func getTxInfo(t *testing.T, block *wire.MsgBlock, txIdx uint) *btcctypes.TransactionInfo {
	mHeaderBytes := bbn.NewBTCHeaderBytesFromBlockHeader(&block.Header)
	var txBytes [][]byte
	for _, tx := range block.Transactions {
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		_ = tx.Serialize(buf)
		txBytes = append(txBytes, buf.Bytes())
	}
	spvProof, err := btcctypes.SpvProofFromHeaderAndTransactions(&mHeaderBytes, txBytes, 1)
	require.NoError(t, err)
	return btcctypes.NewTransactionInfoFromSpvProof(spvProof)
}

// TODO: these functions should be enabled by Babylon
func bbnPksToBtcPks(pks []bbn.BIP340PubKey) ([]*btcec.PublicKey, error) {
	btcPks := make([]*btcec.PublicKey, 0, len(pks))
	for _, pk := range pks {
		btcPk, err := pk.ToBTCPK()
		if err != nil {
			return nil, err
		}
		btcPks = append(btcPks, btcPk)
	}
	return btcPks, nil
}

func outIdx(tx *wire.MsgTx, candOut *wire.TxOut) (uint32, error) {
	for idx, out := range tx.TxOut {
		if bytes.Equal(out.PkScript, candOut.PkScript) && out.Value == candOut.Value {
			return uint32(idx), nil
		}
	}
	return 0, fmt.Errorf("couldn't find output")
}
