// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// TODO: recover the below after we can bump to the latest version of btcd
// var _ chain.Interface = &Client{}

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*rpcclient.Client
	Params *chaincfg.Params
	Cfg    *config.BTCConfig
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

	// TODO: forward notifications to reporter
	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			log.Infof("Block connected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			log.Infof("Block disconnected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
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

	params := netparams.GetBTCParams(cfg.NetParams)
	client := &Client{rpcClient, params, cfg}
	return client, err
}
