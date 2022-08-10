// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"errors"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcwallet/chain"
)

var _ chain.Interface = &Client{}

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*chain.RPCClient
}

// New creates a client connection to the server described by the
// connect string.  If disableTLS is false, the remote RPC certificate must be
// provided in the certs slice.  The connection is not established immediately,
// but must be done using the Start method.  If the remote server does not
// operate on the same bitcoin network as described by the passed chain
// parameters, the connection will be disconnected.
func New(cfg *config.BTCConfig) (*Client, error) {
	if cfg.ReconnectAttempts < 0 {
		return nil, errors.New("reconnectAttempts must be positive")
	}

	certs := readCAFile(cfg)
	params := netparams.GetParams(cfg.NetParams)

	rpcClient, err := chain.NewRPCClient(params.Params, cfg.Endpoint, cfg.Username, cfg.Password, certs, cfg.DisableClientTLS, cfg.ReconnectAttempts)
	if err != nil {
		return nil, err
	}
	client := &Client{rpcClient}
	return client, err
}

func (c *Client) ConnectLoop(cfg *config.BTCConfig) {
	go func() {
		log.Infof("Start connecting to the BTC node %v", cfg.Endpoint)
		if err := c.Start(); err != nil {
			log.Errorf("Unable to connect to the BTC node: %v", err)
		}
		log.Info("Successfully connected to the BTC node")
		c.WaitForShutdown()
	}()
}
