//go:build e2e
// +build e2e

package e2etest

import (
	"bytes"
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
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
	"github.com/stretchr/testify/require"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))
	// BTC validator
	valSK, _, _ = datagen.GenRandomBTCKeyPair(r)
	btcVal, _   = datagen.GenRandomBTCValidatorWithBTCSK(r, valSK)
	// BTC delegation
	delBabylonSK, delBabylonPK, _ = datagen.GenRandomSecp256k1KeyPair(r)
	// jury
	jurySK, _ = btcec.PrivKeyFromBytes(
		[]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	)
	// slashing address
	slashingPkHash     = []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	slashingAddress, _ = btcutil.NewAddressPubKeyHash(slashingPkHash, netParams)
	// staking/slashing tx hash
	stakingMsgTxHash  *chainhash.Hash
	slashingMsgTxHash *chainhash.Hash
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
	tm.createBTCValidatorAndDelegation(t)

	// commit public randomness, vote and equivocate
	tm.voteAndEquivocate(t)

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
	tm.createBTCValidatorAndDelegation(t)

	// commit public randomness, vote and equivocate
	tm.voteAndEquivocate(t)

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
	// bootstrap monitor
	err = vigilanteMonitor.Bootstrap(0)
	require.NoError(t, err)

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

func (tm *TestManager) createBTCValidatorAndDelegation(t *testing.T) {
	ctx := context.Background()
	prefix := tm.BabylonClient.GetConfig().AccountPrefix
	signerAddr := sdk.MustBech32ifyAddressBytes(prefix, tm.BabylonClient.MustGetAddr())

	/*
		create BTC validator
	*/
	msgNewVal := &bstypes.MsgCreateBTCValidator{
		Signer:    signerAddr,
		BabylonPk: btcVal.BabylonPk,
		BtcPk:     btcVal.BtcPk,
		Pop:       btcVal.Pop,
	}
	_, err := tm.BabylonClient.SendMsg(ctx, msgNewVal, "")
	require.NoError(t, err)

	/*
		create BTC delegation
	*/
	// generate staking tx and slashing tx
	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	stakingTimeBlocks := uint16(math.MaxUint16)
	// get top UTXO
	topUnspentResult, _, err := tm.BTCWalletClient.GetHighUTXOAndSum()
	require.NoError(t, err)
	topUTXO, err := types.NewUTXO(topUnspentResult, netParams)
	// staking value
	stakingValue := int64(topUTXO.Amount) / 2
	// dump SK
	wif, err := tm.BTCWalletClient.DumpPrivKey(topUTXO.Addr)
	require.NoError(t, err)
	delBTCSK := wif.PrivKey
	delBTCPK := bbn.NewBIP340PubKeyFromBTCPK(delBTCSK.PubKey())
	// generate legitimate BTC del
	stakingTx, slashingTx, err := datagen.GenBTCStakingSlashingTxWithOutPoint(
		r,
		netParams,
		topUTXO.GetOutPoint(),
		delBTCSK,
		btcVal.BtcPk.MustToBTCPK(),
		bsParams.JuryPk.MustToBTCPK(),
		stakingTimeBlocks,
		stakingValue,
		bsParams.SlashingAddress,
	)
	require.NoError(t, err)
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)

	// sign staking tx and overwrite the staking tx to the signed version
	// NOTE: the tx hash has changed here since stakingMsgTx is pre-segwit
	stakingMsgTx, signed, err := tm.BTCWalletClient.SignRawTransaction(stakingMsgTx)
	require.NoError(t, err)
	require.True(t, signed)
	// overwrite staking tx
	var buf bytes.Buffer
	err = stakingMsgTx.Serialize(&buf)
	stakingTx.Tx = buf.Bytes()
	// get signed staking tx hash
	stakingMsgTxHash1 := stakingMsgTx.TxHash()
	stakingMsgTxHash = &stakingMsgTxHash1
	t.Logf("signed staking tx hash: %s", stakingMsgTxHash.String())

	// change outpoint tx hash of slashing tx to the txhash of the signed staking tx
	slashingMsgTx, err := slashingTx.ToMsgTx()
	require.NoError(t, err)
	slashingMsgTx.TxIn[0].PreviousOutPoint.Hash = stakingMsgTx.TxHash()
	// update slashing tx
	slashingTx, err = bstypes.NewBTCSlashingTxFromMsgTx(slashingMsgTx)
	require.NoError(t, err)
	// get slashing tx hash
	slashingMsgTxHash1 := slashingMsgTx.TxHash()
	slashingMsgTxHash = &slashingMsgTxHash1

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

	// create PoP
	pop, err := bstypes.NewPoP(delBabylonSK, delBTCSK)
	require.NoError(t, err)
	// generate proper delegator sig
	delegatorSig, err := slashingTx.Sign(
		stakingMsgTx,
		stakingTx.StakingScript,
		delBTCSK,
		netParams,
	)
	require.NoError(t, err)

	// catch up BTC light client
	tm.CatchUpBTCLightClient(t)

	// submit BTC delegation to Babylon
	msgBTCDel := &bstypes.MsgCreateBTCDelegation{
		Signer:        signerAddr,
		BabylonPk:     delBabylonPK.(*secp256k1.PubKey),
		Pop:           pop,
		StakingTx:     stakingTx,
		StakingTxInfo: stakingTxInfo,
		SlashingTx:    slashingTx,
		DelegatorSig:  delegatorSig,
	}
	_, err = tm.BabylonClient.SendMsg(ctx, msgBTCDel, "")
	require.NoError(t, err)
	t.Logf("submitted MsgCreateBTCDelegation")

	/*
		generate and insert new jury signature, in order to activate the BTC delegation
	*/
	jurySig, err := slashingTx.Sign(
		stakingMsgTx,
		stakingTx.StakingScript,
		jurySK,
		netParams,
	)
	require.NoError(t, err)
	msgAddJurySig := &bstypes.MsgAddJurySig{
		Signer:        signerAddr,
		ValPk:         btcVal.BtcPk,
		DelPk:         delBTCPK,
		StakingTxHash: stakingTx.MustGetTxHash(),
		Sig:           jurySig,
	}
	_, err = tm.BabylonClient.SendMsg(ctx, msgAddJurySig, "")
	require.NoError(t, err)
	t.Logf("submitted jury signature")

}

func (tm *TestManager) voteAndEquivocate(t *testing.T) {
	ctx := context.Background()
	prefix := tm.BabylonClient.GetConfig().AccountPrefix
	signerAddr := sdk.MustBech32ifyAddressBytes(prefix, tm.BabylonClient.MustGetAddr())

	/*
		commit a number of public randomness since activatedHeight
	*/
	// commit public randomness list
	activatedHeightResp, err := tm.BabylonClient.ActivatedHeight()
	require.NoError(t, err)
	activatedHeight := activatedHeightResp.Height
	srList, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, valSK, activatedHeight, 100)
	require.NoError(t, err)
	msgCommitPubRandList.Signer = signerAddr
	_, err = tm.BabylonClient.SendMsg(ctx, msgCommitPubRandList, "")
	require.NoError(t, err)
	t.Logf("committed public randomness")

	/*
		submit finality signature
	*/
	// get block to vote
	blockToVote, err := tm.BabylonClient.GetBlock(int64(activatedHeight))
	require.NoError(t, err)
	msgToSign := append(sdk.Uint64ToBigEndian(activatedHeight), blockToVote.Block.LastCommitHash...)
	// generate EOTS signature
	sig, err := eots.Sign(valSK, srList[0], msgToSign)
	require.NoError(t, err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)
	// submit finality signature
	msgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:              signerAddr,
		ValBtcPk:            btcVal.BtcPk,
		BlockHeight:         activatedHeight,
		BlockLastCommitHash: blockToVote.Block.LastCommitHash,
		FinalitySig:         eotsSig,
	}
	_, err = tm.BabylonClient.SendMsg(ctx, msgAddFinalitySig, "")
	require.NoError(t, err)
	t.Logf("submitted finality signature")

	/*
		equivocate
	*/
	invalidLch := datagen.GenRandomByteArray(r, 32)
	invalidMsgToSign := append(sdk.Uint64ToBigEndian(activatedHeight), invalidLch...)
	invalidSig, err := eots.Sign(valSK, srList[0], invalidMsgToSign)
	require.NoError(t, err)
	invalidEotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(invalidSig)
	invalidMsgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:              signerAddr,
		ValBtcPk:            btcVal.BtcPk,
		BlockHeight:         activatedHeight,
		BlockLastCommitHash: invalidLch,
		FinalitySig:         invalidEotsSig,
	}
	_, err = tm.BabylonClient.SendMsg(ctx, invalidMsgAddFinalitySig, "")
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
