//go:build e2e
// +build e2e

package e2etest

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/btcstaking"
	"github.com/babylonchain/babylon/crypto/eots"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor"
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
	// BTC validator
	valSK, _, _    = datagen.GenRandomBTCKeyPair(r)
	btcVal, _      = datagen.GenRandomBTCValidatorWithBTCSK(r, valSK)
	btcValBtcPk, _ = btcVal.BtcPk.ToBTCPK()
	// BTC delegation
	delBabylonSK, delBabylonPK, _ = datagen.GenRandomSecp256k1KeyPair(r)

	// covenant
	covenantSk, _ = btcec.PrivKeyFromBytes(
		[]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	)
	// change address
	changeAddress, _ = datagen.GenRandomBTCAddress(r, netParams)
)

func TestMonitor_GracefulShutdown(t *testing.T) {
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

	// create monitor
	genesisInfo := tm.getGenesisInfo(t)
	monitorMetrics := metrics.NewMonitorMetrics()
	tm.Config.Monitor.EnableLivenessChecker = false // we don't test liveness checker in this test case
	vigilanteMonitor, err := monitor.New(
		&tm.Config.Monitor,
		logger,
		genesisInfo,
		tm.BabylonClient.QueryClient,
		tm.BTCClient,
		monitorMetrics,
	)
	// start monitor
	go vigilanteMonitor.Start()
	// wait for bootstrapping
	time.Sleep(10 * time.Second)

	// gracefully shut down
	defer vigilanteMonitor.Stop()
}

func TestMonitor_Slasher(t *testing.T) {
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

	// create monitor
	genesisInfo := tm.getGenesisInfo(t)
	monitorMetrics := metrics.NewMonitorMetrics()
	tm.Config.Monitor.EnableLivenessChecker = false // we don't test liveness checker in this test case
	vigilanteMonitor, err := monitor.New(
		&tm.Config.Monitor,
		logger,
		genesisInfo,
		tm.BabylonClient.QueryClient,
		tm.BTCClient,
		monitorMetrics,
	)
	// start monitor
	go vigilanteMonitor.Start()
	// gracefully shut down at the end
	defer vigilanteMonitor.Stop()

	// wait for bootstrapping
	time.Sleep(5 * time.Second)

	// set up a BTC validator
	tm.createBTCValidator(t)
	// set up a BTC delegation
	stakingSlashingInfo, _ := tm.createBTCDelegation(t)

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

func TestMonitor_SlashingUnbonding(t *testing.T) {
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

	// create monitor
	genesisInfo := tm.getGenesisInfo(t)
	monitorMetrics := metrics.NewMonitorMetrics()
	tm.Config.Monitor.EnableLivenessChecker = false // we don't test liveness checker in this test case
	vigilanteMonitor, err := monitor.New(
		&tm.Config.Monitor,
		logger,
		genesisInfo,
		tm.BabylonClient.QueryClient,
		tm.BTCClient,
		monitorMetrics,
	)
	// start monitor
	go vigilanteMonitor.Start()
	// gracefully shut down at the end
	defer vigilanteMonitor.Stop()

	// wait for bootstrapping
	time.Sleep(5 * time.Second)

	// set up a BTC validator
	tm.createBTCValidator(t)
	// set up a BTC delegation
	_, _ = tm.createBTCDelegation(t)
	// set up a BTC delegation
	stakingSlashingInfo1, stakerPrivKey1 := tm.createBTCDelegation(t)

	// undelegate
	unbondingSlashingInfo, _ := tm.undelegate(t, stakingSlashingInfo1, stakerPrivKey1)

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

func TestMonitor_Bootstrapping(t *testing.T) {
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

	// set up a BTC validator
	tm.createBTCValidator(t)
	// set up a BTC delegation
	stakingSlashingInfo, _ := tm.createBTCDelegation(t)

	// commit public randomness, vote and equivocate
	tm.voteAndEquivocate(t)

	// create monitor
	genesisInfo := tm.getGenesisInfo(t)
	monitorMetrics := metrics.NewMonitorMetrics()
	tm.Config.Monitor.EnableLivenessChecker = false // we don't test liveness checker in this test case
	vigilanteMonitor, err := monitor.New(
		&tm.Config.Monitor,
		logger,
		genesisInfo,
		tm.BabylonClient.QueryClient,
		tm.BTCClient,
		monitorMetrics,
	)
	// bootstrap monitor
	err = vigilanteMonitor.Bootstrap(0)
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

func (tm *TestManager) createBTCValidator(t *testing.T) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	/*
		create BTC validator
	*/
	commission := sdkmath.LegacyZeroDec()
	msgNewVal := &bstypes.MsgCreateBTCValidator{
		Signer:      signerAddr,
		Description: &stakingtypes.Description{},
		Commission:  &commission,
		BabylonPk:   btcVal.BabylonPk,
		BtcPk:       btcVal.BtcPk,
		Pop:         btcVal.Pop,
	}
	_, err := tm.BabylonClient.ReliablySendMsg(context.Background(), msgNewVal, nil, nil)
	require.NoError(t, err)
}

func (tm *TestManager) createBTCDelegation(t *testing.T) (*datagen.TestStakingSlashingInfo, *btcec.PrivateKey) {
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
		[]*btcec.PublicKey{btcValBtcPk},
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

	// catch up BTC light client
	tm.CatchUpBTCLightClient(t)

	// submit BTC delegation to Babylon
	msgBTCDel := &bstypes.MsgCreateBTCDelegation{
		Signer:       signerAddr,
		BabylonPk:    delBabylonPK.(*secp256k1.PubKey),
		Pop:          pop,
		BtcPk:        bbn.NewBIP340PubKeyFromBTCPK(wif.PrivKey.PubKey()),
		ValBtcPkList: []bbn.BIP340PubKey{*btcVal.BtcPk},
		StakingTime:  uint32(stakingTimeBlocks),
		StakingValue: stakingValue,
		StakingTx:    stakingTxInfo,
		SlashingTx:   stakingSlashingInfo.SlashingTx,
		DelegatorSig: delegatorSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgBTCDel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgCreateBTCDelegation")

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// TODO: Make this handle multiple covenant signatures
	encKey, err := asig.NewEncryptionKeyFromBTCPK(valSK.PubKey())
	require.NoError(t, err)
	covenantSig, err := stakingSlashingInfo.SlashingTx.EncSign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		covenantSk,
		encKey,
	)
	require.NoError(t, err)
	msgAddCovenantSig := &bstypes.MsgAddCovenantSig{
		Signer:        signerAddr,
		Pk:            bbn.NewBIP340PubKeyFromBTCPK(covenantSk.PubKey()),
		StakingTxHash: stakingSlashingInfo.StakingTx.TxHash().String(),
		Sigs:          [][]byte{covenantSig.MustMarshal()},
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgAddCovenantSig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted covenant signature")
	return stakingSlashingInfo, wif.PrivKey
}

func (tm *TestManager) undelegate(t *testing.T, stakingSlashingInfo *datagen.TestStakingSlashingInfo, delSK *btcec.PrivateKey) (*datagen.TestUnbondingSlashingInfo, *schnorr.Signature) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	covenantBtcPks, err := bbnPksToBtcPks(bsParams.Params.CovenantPks)
	require.NoError(t, err)
	stakingTimeBlocks := uint16(100)

	fee := int64(1000)
	require.NoError(t, err)
	stakingOutIdx, err := outIdx(stakingSlashingInfo.StakingTx, stakingSlashingInfo.StakingInfo.StakingOutput)
	require.NoError(t, err)
	stakingMsgTxHash1 := stakingSlashingInfo.StakingTx.TxHash()
	stakingMsgTxHash := &stakingMsgTxHash1
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		netParams,
		delSK,
		[]*btcec.PublicKey{btcValBtcPk},
		covenantBtcPks,
		bsParams.Params.CovenantQuorum,
		wire.NewOutPoint(stakingMsgTxHash, stakingOutIdx),
		stakingTimeBlocks,
		stakingSlashingInfo.StakingInfo.StakingOutput.Value-fee,
		bsParams.Params.SlashingAddress,
		changeAddress.String(),
		bsParams.Params.SlashingRate,
	)
	require.NoError(t, err)

	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	unbondingTxSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingSlashingInfo.UnbondingTx,
		stakingSlashingInfo.StakingTx,
		stakingOutIdx,
		unbondingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	unbondingSlashingPathSpendInfo, err := unbondingSlashingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	slashingTxSig, err := unbondingSlashingInfo.SlashingTx.Sign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding tx
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	// Get Unbonding tx bytes
	var unbondingTxBuffer bytes.Buffer
	err = unbondingSlashingInfo.UnbondingTx.Serialize(&unbondingTxBuffer)
	require.NoError(t, err)
	// submit MsgBTCUndelegate
	msgUndel := &bstypes.MsgBTCUndelegate{
		Signer:               signerAddr,
		UnbondingTx:          unbondingTxBuffer.Bytes(),
		UnbondingTime:        uint32(stakingTimeBlocks),
		UnbondingValue:       unbondingSlashingInfo.UnbondingInfo.UnbondingOutput.Value,
		SlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorSlashingSig: slashingTxSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgUndel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgBTCUndelegate")

	// TODO: Add covenant sigs for all covenants
	// add covenant sigs
	// covenant Schnorr sig on unbonding tx
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
	encKey, err := asig.NewEncryptionKeyFromBTCPK(valSK.PubKey())
	require.NoError(t, err)
	covenantSlashingSig, err := unbondingSlashingInfo.SlashingTx.EncSign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding transaction
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		covenantSk,
		encKey,
	)
	require.NoError(t, err)
	msgAddCovenantUnbondingSigs := &bstypes.MsgAddCovenantUnbondingSigs{
		Signer:                  signerAddr,
		Pk:                      bbn.NewBIP340PubKeyFromBTCPK(covenantSk.PubKey()),
		StakingTxHash:           stakingMsgTxHash.String(),
		UnbondingTxSig:          &covenantUnbondingSig,
		SlashingUnbondingTxSigs: [][]byte{covenantSlashingSig.MustMarshal()},
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgAddCovenantUnbondingSigs, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted msgAddCovenantUnbondingSigs")

	// TODO: use multiple covenants
	witness, err := unbondingPathSpendInfo.CreateUnbondingPathWitness(
		[]*schnorr.Signature{unbondingTxCovenantSchnorrSig},
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
	srList, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, valSK, activatedHeight, 100)
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
	sig, err := eots.Sign(valSK, srList[0], msgToSign)
	require.NoError(t, err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)
	// submit finality signature
	msgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		ValBtcPk:     btcVal.BtcPk,
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
	invalidSig, err := eots.Sign(valSK, srList[0], invalidMsgToSign)
	require.NoError(t, err)
	invalidEotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(invalidSig)
	invalidMsgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		ValBtcPk:     btcVal.BtcPk,
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
