package e2etest

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	bbn "github.com/babylonchain/babylon/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

	eventuallyWaitTimeOut = 40 * time.Second
	eventuallyPollTime    = 2 * time.Second
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
	// Config setting necessary to connect btcd daemon
	defaultConfig.BTC.NetParams = "simnet"
	defaultConfig.BTC.Endpoint = "127.0.0.1:18556"
	// Config setting necessary to connect btcwallet daemon
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
	BTCClient        *btcclient.Client
	BTCWalletClient  *btcclient.Client
	Config           *config.Config
}

func initBTCWalletClient(
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

func initBTCClientWithSubscriber(t *testing.T, cfg *config.Config, rpcClient *rpcclient.Client, connCfg *rpcclient.ConnConfig, blockEventChan chan *types.BlockEvent) *btcclient.Client {
	btcCfg := &config.BTCConfig{
		NetParams:        cfg.BTC.NetParams,
		Username:         connCfg.User,
		Password:         connCfg.Pass,
		Endpoint:         connCfg.Host,
		DisableClientTLS: connCfg.DisableTLS,
	}
	client, err := btcclient.NewTestClientWithWsSubscriber(rpcClient, btcCfg, cfg.Common.RetrySleepTime, cfg.Common.MaxRetrySleepTime, blockEventChan)
	require.NoError(t, err)

	// lets wait until chain rpc becomes available
	// poll time is increase here to avoid spamming the rpc server
	require.Eventually(t, func() bool {
		if _, _, err := client.GetBestBlock(); err != nil {
			log.Errorf("failed to get best block: %v", err)
			return false
		}

		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return client
}

// StartManager creates a test manager
// NOTE: if handlers.OnFilteredBlockConnected, handlers.OnFilteredBlockDisconnected
// and blockEventChan are all not nil, then the test manager will create a BTC
// client with a WebSocket subscriber
func StartManager(
	t *testing.T,
	numMatureOutputsInWallet uint32,
	numbersOfOutputsToWaitForDuringInit int,
	handlers *rpcclient.NotificationHandlers,
	blockEventChan chan *types.BlockEvent) *TestManager {
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

	var btcClient *btcclient.Client
	if handlers.OnFilteredBlockConnected != nil && handlers.OnFilteredBlockDisconnected != nil {
		// BTC client with subscriber
		btcClient = initBTCClientWithSubscriber(t, cfg, miner.Client, &minerNodeRpcConfig, blockEventChan)
	}
	// we always want BTC wallet client for sending txs
	btcWalletClient := initBTCWalletClient(
		t,
		cfg,
		privkey,
		numbersOfOutputsToWaitForDuringInit,
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
		BTCClient:        btcClient,
		BTCWalletClient:  btcWalletClient,
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
func (tm *TestManager) MineBlockWithTxs(t *testing.T, txs []*btcutil.Tx) *wire.MsgBlock {
	var emptyTime time.Time

	// Generate a block.
	b, err := tm.MinerNode.GenerateAndSubmitBlock(txs, -1, emptyTime)
	require.NoError(t, err, "unable to mine block")

	block, err := tm.MinerNode.Client.GetBlock(b.Hash())
	require.NoError(t, err, "unable to get block")

	return block
}

func (tm *TestManager) MustGetBabylonSigner() string {
	prefix := tm.BabylonClient.GetConfig().AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, tm.BabylonClient.MustGetAddr())
}

func (tm *TestManager) RetrieveTransactionFromMempool(t *testing.T, hashes []*chainhash.Hash) []*btcutil.Tx {
	var txes []*btcutil.Tx
	for _, txHash := range hashes {
		tx, err := tm.BTCClient.GetRawTransaction(txHash)
		require.NoError(t, err)
		txes = append(txes, tx)
	}
	return txes
}

func (tm *TestManager) InsertBTCHeadersToBabylon(headers []*wire.BlockHeader) (*sdk.TxResponse, error) {
	var headersBytes []bbn.BTCHeaderBytes

	for _, h := range headers {
		headersBytes = append(headersBytes, bbn.NewBTCHeaderBytesFromBlockHeader(h))
	}

	msg := btclctypes.MsgInsertHeaders{
		Headers: headersBytes,
		Signer:  tm.MustGetBabylonSigner(),
	}

	return tm.BabylonClient.InsertHeaders(&msg)
}

func (tm *TestManager) CatchUpBTCLightClient(t *testing.T) {
	_, btcHeight, err := tm.MinerNode.Client.GetBestBlock()
	require.NoError(t, err)

	tipResp, err := tm.BabylonClient.BTCHeaderChainTip()
	require.NoError(t, err)
	btclcHeight := tipResp.Header.Height

	headers := []*wire.BlockHeader{}
	for i := int(btclcHeight + 1); i <= int(btcHeight); i++ {
		hash, err := tm.MinerNode.Client.GetBlockHash(int64(i))
		require.NoError(t, err)
		header, err := tm.MinerNode.Client.GetBlockHeader(hash)
		require.NoError(t, err)
		headers = append(headers, header)
	}

	_, err = tm.InsertBTCHeadersToBabylon(headers)
	require.NoError(t, err)
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
