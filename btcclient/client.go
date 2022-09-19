// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
)

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*rpcclient.Client
	Params *chaincfg.Params
	Cfg    *config.BTCConfig

	// Keep track of hash/height of latest block in canonical chain
	// TODO: only used in poller at the moment. extend to all clients
	lastBlockHash   *chainhash.Hash
	lastBlockHeight int32

	// channels for notifying new BTC blocks to reporter
	IndexedBlockChan chan *types.IndexedBlock
}

func (c *Client) Stop() {
	c.Shutdown()
	close(c.IndexedBlockChan)
}
