package btcclient

import (
	"time"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/rpcclient"
)

// NewWithBlockPoller creates a new BTC client that polls new blocks from BTC
func NewWithBlockPoller(cfg *config.BTCConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client := &Client{}
	params := netparams.GetBTCParams(cfg.NetParams)
	client.IndexedBlockChan = make(chan *types.IndexedBlock, 10000) // TODO: parameterise buffer size
	client.Cfg = cfg
	client.Params = params

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Endpoint,
		HTTPPostMode: true, // Poller uses HTTP rather than Websocket
		User:         cfg.Username,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableClientTLS,
		Params:       cfg.NetParams,
		Certificates: readCAFile(cfg),
	}

	rpcClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}
	client.Client = rpcClient
	log.Info("Successfully created the BTC client and connected to the BTC server")

	// Retrieve hash/height of the latest block in BTC
	client.LastBlockHash, client.LastBlockHeight, err = client.GetBestBlock()
	if err != nil {
		panic(err)
	}

	return client, nil
}

func (c *Client) SubscribeBlocksByPolling() {
	go c.blockPoller()
	log.Info("Successfully subscribed to newly connected blocks via polling")
}

func (c *Client) blockPoller() {
	// TODO: parameterise poll frequency
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		// Retrieve hash/height of the latest block in BTC
		lastBlockHash, lastBlockHeight, err := c.GetBestBlock()
		if err != nil {
			panic(err)
		}
		log.Infof("BTC latest block hash and height: (%v, %d)", lastBlockHash, lastBlockHeight)

		if c.LastBlockHeight >= lastBlockHeight {
			log.Info("No new block in this polling attempt")
			continue
		}

		// TODO: detect reorg

		syncHeight := uint64(c.LastBlockHeight + 1)
		ibs, err := c.GetLastBlocks(syncHeight)
		if err != nil {
			panic(err)
		}
		log.Infof("BTC client falls behind BTC by %d blocks.", len(ibs))

		for _, ib := range ibs {
			c.IndexedBlockChan <- ib
			log.Infof("New latest block: hash: %v, height: %d.", ib.BlockHash(), ib.Height)
		}

		// refresh last block info
		c.LastBlockHash, c.LastBlockHeight = lastBlockHash, lastBlockHeight
	}
}
