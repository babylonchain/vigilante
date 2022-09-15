// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

// TODO: recover the below after we can bump to the latest version of btcd
// var _ chain.Interface = &Client{}

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*rpcclient.Client
	Params *chaincfg.Params
	Cfg    *config.BTCConfig

	// channels for notifying new BTC blocks to reporter
	IndexedBlockChan chan *types.IndexedBlock
}

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
		Params:       cfg.NetParams,
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

// NewWithBlockNotificationHandlers creates a new BTC client that subscribes to newly connected/disconnected blocks
// used by vigilant reporter
func NewWithBlockNotificationHandlers(cfg *config.BTCConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	params := netparams.GetBTCParams(cfg.NetParams)
	client := &Client{}
	client.IndexedBlockChan = make(chan *types.IndexedBlock, 1000) // TODO: parameterise buffer size
	client.Cfg = cfg
	client.Params = params

	notificationHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			log.Debugf("Block %v with %d txs at height %d has been connected at time %v", header.BlockHash(), len(txs), height, header.Timestamp)
			client.IndexedBlockChan <- types.NewIndexedBlock(height, header, txs) // TODO: bug here
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			log.Debugf("Block %v at height %d has been disconnected at time %v", header.BlockHash(), height, header.Timestamp)
			// TODO: should we notify BTCLightClient here?
		},
	}

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Endpoint,
		Endpoint:     "ws", // websocket
		User:         cfg.Username,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableClientTLS,
		Params:       cfg.NetParams,
		Certificates: readCAFile(cfg),
	}

	rpcClient, err := rpcclient.New(connCfg, &notificationHandlers)
	if err != nil {
		return nil, err
	}
	log.Info("Successfully created the BTC client and connected to the BTC server")

	if err := rpcClient.NotifyBlocks(); err != nil {
		return nil, err
	}
	log.Info("Successfully subscribed to newly connected/disconnected blocks from BTC")

	client.Client = rpcClient
	return client, nil
}

func (c *Client) Stop() {
	c.Shutdown()
	close(c.IndexedBlockChan)
}
