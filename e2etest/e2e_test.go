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
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/submitter"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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

	existingWalletFile = "wallet.db"
	exisitngWalletPass = "pass"
	walletTimeout      = 86400

	eventuallyWaitTimeOut = 10 * time.Second
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

func GetSpendingKeyAndAddress() (*btcec.PrivateKey, btcutil.Address, error) {
	var harnessHDSeed [chainhash.HashSize + 4]byte
	copy(harnessHDSeed[:], hdSeed[:])
	// id used for our test wallet is always 0
	binary.BigEndian.PutUint32(harnessHDSeed[:chainhash.HashSize], 0)

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
}

func StartManager(t *testing.T, numMatureOutputsInWallet uint32) *TestManager {
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

	miner, err := rpctest.New(netParams, nil, args, "")
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

	defaultConfig := config.DefaultConfig()
	// Config setting necessary to connect btcwwallet daemon
	defaultConfig.BTC.BtcBackend = "btcd"
	defaultConfig.BTC.WalletEndpoint = "127.0.0.1:18554"
	defaultConfig.BTC.WalletPassword = "pass"
	defaultConfig.BTC.Username = "user"
	defaultConfig.BTC.Password = "pass"
	defaultConfig.BTC.DisableClientTLS = true

	return &TestManager{
		MinerNode:        miner,
		BtcWalletHandler: wh,
		Config:           defaultConfig,
	}
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.BtcWalletHandler.Stop()
	require.NoError(t, err)
	err = tm.MinerNode.TearDown()
	require.NoError(t, err)
}

func ImportWalletSpendingKey(t *testing.T, walletClient *btcclient.Client) {
	privKey, _, err := GetSpendingKeyAndAddress()
	require.NoError(t, err)

	wifKey, err := btcutil.NewWIF(privKey, netParams, true)
	require.NoError(t, err)

	err = walletClient.WalletPassphrase(exisitngWalletPass, int64(walletTimeout))
	require.NoError(t, err)

	err = walletClient.ImportPrivKey(wifKey)
	require.NoError(t, err)
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

func retrieveNTransactionFromMempool(t *testing.T, h *rpctest.Harness, n int) []*btcutil.Tx {
	var txHashes []*chainhash.Hash
	require.Eventually(t, func() bool {
		transactions, err := h.Client.GetRawMempool()
		require.NoError(t, err)

		if len(transactions) != n {
			return false
		}

		txHashes = transactions
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	var txes []*btcutil.Tx
	for _, txHash := range txHashes {
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
	tm := StartManager(t, numMatureOutputs)
	// wait for wallet daemon to start, ultimately our client should have reconnecting logic
	// with some backoff and max re-tries
	time.Sleep(3 * time.Second)
	defer tm.Stop(t)

	btcWallet, err := btcclient.NewWallet(&tm.Config.BTC)
	require.NoError(t, err)

	ImportWalletSpendingKey(t, btcWallet)
	waitForNOutputs(t, btcWallet, 1)

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
		btcWallet,
		mockBabylonClient,
		subAddr,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		metrics.NewSubmitterMetrics(),
	)

	vigilantSubmitter.Start()
	defer vigilantSubmitter.Stop()

	// wait for our 2 op_returns with epoch 1 checkpoint to hit the mempool and then
	// retrieve them from there
	//
	// TODO: to assert that those are really transactions send by submitter, we would
	// need to expose sentCheckpointInfo from submitter
	sendTransactions := retrieveNTransactionFromMempool(t, tm.MinerNode, 2)
	require.Equal(t, len(sendTransactions), 2)

	// mine a block with those transactions
	blockWithOpReturnTranssactions := mineBlockWithTxes(t, tm.MinerNode, sendTransactions)
	// block should have 3 transactions, 2 from submitter and 1 coinbase
	require.Equal(t, len(blockWithOpReturnTranssactions.Transactions), 3)
}
