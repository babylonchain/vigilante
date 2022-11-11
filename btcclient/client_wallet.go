package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// NewWallet creates a new BTC wallet
// used by vigilant submitter
// a wallet is essentially a BTC client
// that connects to the btcWallet daemon
func NewWallet(cfg *config.BTCConfig) (*Client, error) {
	params := netparams.GetBTCParams(cfg.NetParams)
	wallet := &Client{}
	wallet.Cfg = cfg
	wallet.Params = params

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.WalletEndpoint,
		Endpoint:     "ws", // websocket
		User:         cfg.Username,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableClientTLS,
		Params:       params.Name,
		Certificates: readWalletCAFile(cfg),
	}

	rpcClient, err := rpcclient.New(connCfg, nil) // TODO: subscribe to wallet stuff?
	if err != nil {
		return nil, err
	}
	log.Info("Successfully connected to the BTC wallet server")

	wallet.Client = rpcClient

	// load wallet from config
	err = wallet.loadWallet(cfg.WalletName)
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func (c *Client) loadWallet(name string) error {
	backend, err := c.BackendVersion()
	if err != nil {
		return err
	}
	// if the backend is btcd, no need to load wallet
	if backend == rpcclient.Btcd {
		log.Infof("BTC backend is btcd")
		return nil
	}

	log.Infof("BTC backend is bitcoind")

	// this is for bitcoind
	res, err := c.Client.LoadWallet(name)
	if err != nil {
		return err
	}
	log.Infof("Successfully loaded wallet %v", res.Name)
	if res.Warning != "" {
		log.Infof("Warning: %v", res.Warning)
	}
	return nil
}

// TODO make it dynamic
func (c *Client) GetTxFee() uint64 {
	return uint64(c.Cfg.TxFee.ToUnit(btcutil.AmountSatoshi))
}

func (c *Client) GetWalletName() string {
	return c.Cfg.WalletName
}

func (c *Client) GetWalletPass() string {
	return c.Cfg.WalletPassword
}

func (c *Client) GetWalletLockTime() int64 {
	return c.Cfg.WalletLockTime
}

func (c *Client) GetNetParams() *chaincfg.Params {
	return netparams.GetBTCParams(c.Cfg.NetParams)
}

func (c *Client) ListUnspent() ([]btcjson.ListUnspentResult, error) {
	return c.Client.ListUnspent()
}

func (c *Client) SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error) {
	return c.Client.SendRawTransaction(tx, allowHighFees)
}

func (c *Client) ListReceivedByAddress() ([]btcjson.ListReceivedByAddressResult, error) {
	return c.Client.ListReceivedByAddress()
}

func (c *Client) GetRawChangeAddress(account string) (btcutil.Address, error) {
	return c.Client.GetRawChangeAddress(account)
}

func (c *Client) WalletPassphrase(passphrase string, timeoutSecs int64) error {
	return c.Client.WalletPassphrase(passphrase, timeoutSecs)
}

func (c *Client) DumpPrivKey(address btcutil.Address) (*btcutil.WIF, error) {
	return c.Client.DumpPrivKey(address)
}
