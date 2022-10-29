// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"
	"github.com/babylonchain/vigilante/zmq"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
)

var _ BTCClient = &Client{}

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*rpcclient.Client
	Params *chaincfg.Params
	Cfg    *config.BTCConfig

	// retry attributes
	retrySleepTime    time.Duration
	maxRetrySleepTime time.Duration

	// channel for notifying new BTC blocks to reporter
	BlockEventChan chan *types.BlockEvent

	ZmqSequenceMsgChan chan *zmq.SequenceMsg
}

func (c *Client) Stop() {
	c.Shutdown()
	close(c.BlockEventChan)
}
