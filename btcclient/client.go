// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/chain"
)

// var _ chain.Interface = &Client{}

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*chain.RPCClient
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

	certs := readCAFile(cfg)
	params := netparams.GetBTCParams(cfg.NetParams)

	rpcClient, err := chain.NewRPCClient(params, cfg.Endpoint, cfg.Username, cfg.Password, certs, cfg.DisableClientTLS, cfg.ReconnectAttempts)
	if err != nil {
		return nil, err
	}
	client := &Client{rpcClient, params, cfg}
	log.Infof("Successfully created the BTC client")
	return client, err
}

func (c *Client) ConnectLoop() {
	go func() {
		log.Infof("Start connecting to the BTC node %v", c.Cfg.Endpoint)
		if err := c.Start(); err != nil {
			log.Errorf("Unable to connect to the BTC node: %v", err)
		}
		log.Info("Successfully connected to the BTC node")
		c.WaitForShutdown()
	}()
}
