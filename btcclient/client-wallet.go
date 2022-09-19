package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"

	"github.com/btcsuite/btcd/rpcclient"
)

// NewWallet creates a new BTC wallet
// used by vigilant submitter
// a wallet is essentially a BTC client
// that connects to the btcWallet daemon
func NewWallet(cfg *config.BTCConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

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

func (cli *Client) loadWallet(name string) error {
	backend, err := cli.BackendVersion()
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
	res, err := cli.Client.LoadWallet(name)
	if err != nil {
		return err
	}
	log.Infof("Successfully loaded wallet %v", res.Name)
	if res.Warning != "" {
		log.Infof("Warning: %v", res.Warning)
	}
	return nil
}
