// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcjson"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"
	"github.com/babylonchain/vigilante/zmq"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
)

var _ BTCClient = &Client{}

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*rpcclient.Client
	zmqClient *zmq.Client

	Params *chaincfg.Params
	Cfg    *config.BTCConfig

	// retry attributes
	retrySleepTime    time.Duration
	maxRetrySleepTime time.Duration

	// channel for notifying new BTC blocks to reporter
	blockEventChan chan *types.BlockEvent
}

func (c *Client) GetTipBlockVerbose() (*btcjson.GetBlockVerboseResult, error) {
	tipBtcHash, err := c.GetBestBlockHash()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain BTC block tip: %w", err)
	}
	tipBlock, err := c.GetBlockVerbose(tipBtcHash)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain BTC tip block: %w", err)
	}

	return tipBlock, nil
}

func (c *Client) Stop() {
	c.Shutdown()
	// NewWallet will create a client with nil blockEventChan,
	// while NewWithBlockSubscriber will have a non-nil one, so
	// we need to check here
	if c.blockEventChan != nil {
		close(c.blockEventChan)
	}
}
