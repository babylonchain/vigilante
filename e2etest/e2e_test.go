//go:build e2e
// +build e2e

package e2etest

import (
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"

	"github.com/babylonchain/babylon/testutil/datagen"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/babylonchain/rpc-client/testutil/mocks"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/submitter"
)

// bticoin params used for testing
var (
	netParams        = &chaincfg.SimNetParams
	submitterAddrStr = "bbn1eppc73j56382wjn6nnq3quu5eye4pmm087xfdh"
	babylonTag       = []byte{1, 2, 3, 4}
	babylonTagHex    = hex.EncodeToString(babylonTag)

	// copy of the seed from btcd/integration/rpctest memWallet, this way we can
	// import the same wallet in the btcd wallet
	hdSeed = [chainhash.HashSize]byte{
		0x79, 0xa6, 0x1a, 0xdb, 0xc6, 0xe5, 0xa2, 0xe1,
		0x39, 0xd2, 0x71, 0x3a, 0x54, 0x6e, 0xc7, 0xc8,
		0x75, 0x63, 0x2e, 0x75, 0xf1, 0xdf, 0x9c, 0x3f,
		0xa6, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	// current number of active test nodes. This is necessary to replicate btcd rpctest.Harness
	// methods of generating keys i.e with each started btcd node we increment this number
	// by 1, and then use hdSeed || numTestInstances as the seed for generating keys
	numTestInstances = 0

	existingWalletFile = "wallet.db"
	exisitngWalletPass = "pass"
	walletTimeout      = 86400

	eventuallyWaitTimeOut = 20 * time.Second
	eventuallyPollTime    = 500 * time.Millisecond
)

// keyToAddr maps the passed private to corresponding p2pkh address.
func keyToAddr(key *btcec.PrivateKey, net *chaincfg.Params) (btcutil.Address, error) {
	serializedKey := key.PubKey().SerializeCompressed()
	pubKeyAddr, err := btcutil.NewAddressPubKey(serializedKey, net)
	if err != nil {
		return nil, err
	}
	return pubKeyAddr.AddressPubKeyHash(), nil
}

func defaultBtcwalletClientConfig() *config.Config {
	defaultConfig := config.DefaultConfig()
	// Config setting necessary to connect btcwwallet daemon
	defaultConfig.BTC.BtcBackend = "btcd"
	defaultConfig.BTC.WalletEndpoint = "127.0.0.1:18554"
	defaultConfig.BTC.WalletPassword = "pass"
	defaultConfig.BTC.Username = "user"
	defaultConfig.BTC.Password = "pass"
	defaultConfig.BTC.DisableClientTLS = true
	return defaultConfig
}

func GetSpendingKeyAndAddress(id uint32) (*btcec.PrivateKey, btcutil.Address, error) {
	var harnessHDSeed [chainhash.HashSize + 4]byte
	copy(harnessHDSeed[:], hdSeed[:])
	// id used for our test wallet is always 0
	binary.BigEndian.PutUint32(harnessHDSeed[:chainhash.HashSize], id)

	hdRoot, err := hdkeychain.NewMaster(harnessHDSeed[:], netParams)

	if err != nil {
		return nil, nil, err
	}

	// The first child key from the hd root is reserved as the coinbase
	// generation address.
	coinbaseChild, err := hdRoot.Derive(0)
	if err != nil {
		return nil, nil, err
	}

	coinbaseKey, err := coinbaseChild.ECPrivKey()

	if err != nil {
		return nil, nil, err
	}

	coinbaseAddr, err := keyToAddr(coinbaseKey, netParams)
	if err != nil {
		return nil, nil, err
	}

	return coinbaseKey, coinbaseAddr, nil
}

type TestManager struct {
	MinerNode        *rpctest.Harness
	BtcWalletHandler *WalletHandler
	Config           *config.Config
	BtcWalletClient  *btcclient.Client
}

func initBtcWalletClient(
	t *testing.T,
	cfg *config.Config,
	walletPrivKey *btcec.PrivateKey,
	outputsToWaitFor int) *btcclient.Client {

	var client *btcclient.Client

	require.Eventually(t, func() bool {
		btcWallet, err := btcclient.NewWallet(&cfg.BTC)
		if err != nil {
			return false
		}

		client = btcWallet
		return true

	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// lets wait until chain rpc becomes available
	// poll time is increase here to avoid spamming the btcwallet rpc server
	require.Eventually(t, func() bool {
		_, _, err := client.GetBestBlock()

		if err != nil {
			return false
		}

		return true
	}, eventuallyWaitTimeOut, 1*time.Second)

	err := ImportWalletSpendingKey(t, client, walletPrivKey)
	require.NoError(t, err)

	waitForNOutputs(t, client, outputsToWaitFor)

	return client
}

func StartManager(
	t *testing.T,
	numMatureOutputsInWallet uint32,
	numbersOfOutputsToWaitForDurintInit int,
	handlers *rpcclient.NotificationHandlers) *TestManager {
	args := []string{
		"--rejectnonstd",
		"--txindex",
		"--trickleinterval=100ms",
		"--debuglevel=debug",
		"--nowinservice",
		// The miner will get banned and disconnected from the node if
		// its requested data are not found. We add a nobanning flag to
		// make sure they stay connected if it happens.
		"--nobanning",
		// Don't disconnect if a reply takes too long.
		"--nostalldetect",
	}

	miner, err := rpctest.New(netParams, handlers, args, "")
	require.NoError(t, err)

	privkey, _, err := GetSpendingKeyAndAddress(uint32(numTestInstances))
	require.NoError(t, err)

	if err := miner.SetUp(true, numMatureOutputsInWallet); err != nil {
		t.Fatalf("unable to set up mining node: %v", err)
	}

	minerNodeRpcConfig := miner.RPCConfig()
	certFile := minerNodeRpcConfig.Certificates

	currentDir, err := os.Getwd()
	require.NoError(t, err)
	walletPath := filepath.Join(currentDir, existingWalletFile)

	wh, err := NewWalletHandler(certFile, walletPath, minerNodeRpcConfig.Host)
	require.NoError(t, err)

	err = wh.Start()
	require.NoError(t, err)

	cfg := defaultBtcwalletClientConfig()

	btcWalletClient := initBtcWalletClient(
		t,
		cfg,
		privkey,
		numbersOfOutputsToWaitForDurintInit,
	)

	numTestInstances++

	return &TestManager{
		MinerNode:        miner,
		BtcWalletHandler: wh,
		Config:           cfg,
		BtcWalletClient:  btcWalletClient,
	}
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.BtcWalletHandler.Stop()
	require.NoError(t, err)
	err = tm.MinerNode.TearDown()
	require.NoError(t, err)
}

func ImportWalletSpendingKey(
	t *testing.T,
	walletClient *btcclient.Client,
	privKey *btcec.PrivateKey) error {

	wifKey, err := btcutil.NewWIF(privKey, netParams, true)
	require.NoError(t, err)

	err = walletClient.WalletPassphrase(exisitngWalletPass, int64(walletTimeout))

	if err != nil {
		return err
	}

	err = walletClient.ImportPrivKey(wifKey)

	if err != nil {
		return err
	}

	return nil
}

// MineBlocksWithTxes mines a single block to include the specifies
// transactions only.
func mineBlockWithTxes(t *testing.T, h *rpctest.Harness, txes []*btcutil.Tx) *wire.MsgBlock {
	var emptyTime time.Time

	// Generate a block.
	b, err := h.GenerateAndSubmitBlock(txes, -1, emptyTime)
	require.NoError(t, err, "unable to mine block")

	block, err := h.Client.GetBlock(b.Hash())
	require.NoError(t, err, "unable to get block")

	return block
}

func retrieveTransactionFromMempool(t *testing.T, h *rpctest.Harness, hashes []*chainhash.Hash) []*btcutil.Tx {
	var txes []*btcutil.Tx
	for _, txHash := range hashes {
		tx, err := h.Client.GetRawTransaction(txHash)
		require.NoError(t, err)
		txes = append(txes, tx)
	}
	return txes
}

func waitForNOutputs(t *testing.T, walletClient *btcclient.Client, n int) {
	require.Eventually(t, func() bool {
		outputs, err := walletClient.ListUnspent()

		if err != nil {
			return false
		}

		return len(outputs) >= n
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

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

	tm := StartManager(t, numMatureOutputs, 2, handlers)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	defer tm.Stop(t)

	randomCheckpoint := datagen.GenRandomRawCheckpointWithMeta(r)
	randomCheckpoint.Status = checkpointingtypes.Sealed
	randomCheckpoint.Ckpt.EpochNum = 1

	ctl := gomock.NewController(t)
	mockBabylonClient := mocks.NewMockBabylonQueryClient(ctl)
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
		tm.BtcWalletClient,
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

	sendTransactions := retrieveTransactionFromMempool(t, tm.MinerNode, submittedTransactions)
	// mine a block with those transactions
	blockWithOpReturnTranssactions := mineBlockWithTxes(t, tm.MinerNode, sendTransactions)
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

	tm := StartManager(t, numMatureOutputs, 2, handlers)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	defer tm.Stop(t)

	randomCheckpoint := datagen.GenRandomRawCheckpointWithMeta(r)
	randomCheckpoint.Status = checkpointingtypes.Sealed
	randomCheckpoint.Ckpt.EpochNum = 1

	ctl := gomock.NewController(t)
	mockBabylonClient := mocks.NewMockBabylonQueryClient(ctl)
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
		tm.BtcWalletClient,
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

	sendTransactions := retrieveTransactionFromMempool(t, tm.MinerNode, submittedTransactions)

	// at this point our submitter already sent 2 checkpoint transactions which landed in mempool.
	// Zero out submittedTransactions, and wait for a new tx2 to be submitted and accepted
	// it should be replacements for the previous one.
	submittedTransactions = []*chainhash.Hash{}

	require.Eventually(t, func() bool {
		// we only replace tx2 of the checkpoint, thus waiting for 1 tx to arrive
		return len(submittedTransactions) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	transactionReplacement := retrieveTransactionFromMempool(t, tm.MinerNode, submittedTransactions)
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
	blockWithOpReturnTransactions := mineBlockWithTxes(t, tm.MinerNode, sendTransactions)
	// block should have 2 transactions, 1 from submitter and 1 coinbase
	require.Equal(t, len(blockWithOpReturnTransactions.Transactions), 3)
}
