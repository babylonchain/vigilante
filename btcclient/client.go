// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"

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

	// channels for notifying the vigilante reporter
	IndexedBlockChan chan *IndexedBlock
}

// New creates a client connection to the server described by the
// connect string.  If disableTLS is false, the remote RPC certificate must be
// provided in the certs slice.  The connection is not established immediately,
// but must be done using the Start method.  If the remote server does not
// operate on the same bitcoin network as described by the passed chain
// parameters, the connection will be disconnected.
func New(cfg *config.BTCConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	params := netparams.GetBTCParams(cfg.NetParams)
	client := &Client{}
	client.IndexedBlockChan = make(chan *IndexedBlock)
	client.Cfg = cfg
	client.Params = params

	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			log.Debugf("Block %v at height %d has been connected at time %v", header.BlockHash(), height, header.Timestamp)
			client.IndexedBlockChan <- NewIndexedBlock(height, header, txs)
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

	rpcClient, err := rpcclient.New(connCfg, &ntfnHandlers)
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
