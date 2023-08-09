package e2etest

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

// bticoin params used for testing
var (
	netParams        = &chaincfg.SimNetParams
	submitterAddrStr = "bbn1eppc73j56382wjn6nnq3quu5eye4pmm087xfdh" //nolint:unused
	babylonTag       = []byte{1, 2, 3, 4}                           //nolint:unused
	babylonTagHex    = hex.EncodeToString(babylonTag)               //nolint:unused

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

	eventuallyWaitTimeOut = 10 * time.Second
	eventuallyPollTime    = 1 * time.Second
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

func defaultVigilanteConfig() *config.Config {
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
	BabylonHandler   *BabylonNodeHandler
	BabylonClient    *bbnclient.Client
	BtcWalletClient  *btcclient.Client
	Config           *config.Config
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
		if _, _, err := client.GetBestBlock(); err != nil {
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

	// start Bitcoin wallet
	wh, err := NewWalletHandler(certFile, walletPath, minerNodeRpcConfig.Host)
	require.NoError(t, err)
	err = wh.Start()
	require.NoError(t, err)

	// Wait for wallet to re-index the outputs
	time.Sleep(5 * time.Second)

	cfg := defaultVigilanteConfig()

	btcWalletClient := initBtcWalletClient(
		t,
		cfg,
		privkey,
		numbersOfOutputsToWaitForDurintInit,
	)

	// start Babylon node
	bh, err := NewBabylonNodeHandler()
	require.NoError(t, err)
	err = bh.Start()
	require.NoError(t, err)
	// create Babylon client
	cfg.Babylon.KeyDirectory = bh.GetNodeDataDir()
	cfg.Babylon.Key = "test-spending-key"
	cfg.Babylon.GasAdjustment = 3.0
	babylonClient, err := bbnclient.New(&cfg.Babylon)
	require.NoError(t, err)
	// wait until Babylon is ready
	require.Eventually(t, func() bool {
		resp, err := babylonClient.CurrentEpoch()
		if err != nil {
			return false
		}
		log.Infof("Babylon is ready. CurrentEpoch: %v", resp)
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	numTestInstances++

	return &TestManager{
		MinerNode:        miner,
		BtcWalletHandler: wh,
		BabylonHandler:   bh,
		BabylonClient:    babylonClient,
		BtcWalletClient:  btcWalletClient,
		Config:           cfg,
	}
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.BtcWalletHandler.Stop()
	require.NoError(t, err)
	err = tm.MinerNode.TearDown()
	require.NoError(t, err)
	if tm.BabylonClient.IsRunning() {
		tm.BabylonClient.Stop()
	}
	err = tm.BabylonHandler.Stop()
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
//
//nolint:unused
func mineBlockWithTxes(t *testing.T, h *rpctest.Harness, txes []*btcutil.Tx) *wire.MsgBlock {
	var emptyTime time.Time

	// Generate a block.
	b, err := h.GenerateAndSubmitBlock(txes, -1, emptyTime)
	require.NoError(t, err, "unable to mine block")

	block, err := h.Client.GetBlock(b.Hash())
	require.NoError(t, err, "unable to get block")

	return block
}

//nolint:unused
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
