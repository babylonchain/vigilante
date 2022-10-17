package btcclient

import (
	"encoding/hex"
	"fmt"
	"github.com/joakimofv/go-bitcoindclient/v23"
	"time"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/rpcclient"
)

// NewWithBlockPoller creates a new BTC client that polls new blocks from BTC
func NewWithBlockPoller(cfg *config.BTCConfig, retrySleepTime, maxRetrySleepTime time.Duration) (*Client, error) {
	client := &Client{}
	params := netparams.GetBTCParams(cfg.NetParams)
	client.IndexedBlockChan = make(chan *types.IndexedBlock, 10000) // TODO: parameterise buffer size
	client.Cfg = cfg
	client.Params = params

	client.retrySleepTime = retrySleepTime
	client.maxRetrySleepTime = maxRetrySleepTime

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Endpoint,
		HTTPPostMode: true, // Poller uses HTTP rather than Websocket
		User:         cfg.Username,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableClientTLS,
		Params:       params.Name,
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
		return nil, err
	}

	return client, nil
}

func (c *Client) mustSubscribeBlocksByPolling() {
	// make sure you add this to your bitcoin.conf, check https://bitcoindev.network/accessing-bitcoins-zeromq-interface
	// zmqpubrawblock=tcp://127.0.0.1:29000
	// zmqpubrawtx=tcp://127.0.0.1:29000
	// zmqpubhashtx=tcp://127.0.0.1:29000
	// zmqpubhashblock=tcp://127.0.0.1:29000

	// use 18332 for testnet, 18443 for regtest
	bc, err := bitcoindclient.New(bitcoindclient.Config{
		RpcAddress:    "localhost:18443",
		RpcUser:       "rpcuser",
		RpcPassword:   "rpcpass",
		ZmqPubAddress: "tcp://localhost:29000",
	})
	if err != nil {
		panic(err)
	}
	//go c.blockPoller()

	for {
		if err := bc.Ready(); err != nil {
			panic(err)
		} else {
			// Success!
			break
		}
	}

	blockCh, _, err := bc.SubscribeHashBlock()
	if err != nil {
		panic(err)
	}

	seqCh, _, err := bc.SubscribeSequence()
	if err != nil {
		panic(err)
	}
	go c.blockReceiver(blockCh)
	go c.sequenceReceiver(seqCh)
	log.Info("Successfully subscribed to newly connected blocks via polling")
}

// TODO: change all queries to Must-style
func (c *Client) blockReceiver(ch chan bitcoindclient.HashMsg) {
	select {
	case msg, open := <-ch:
		if !open {
			// return
		}
		fmt.Println("received new block notification via zmq", hex.EncodeToString(msg.Hash[:]))
	}
}

// TODO: change all queries to Must-style
func (c *Client) sequenceReceiver(ch1 chan bitcoindclient.SequenceMsg) {
	select {
	case msg, open := <-ch1:
		if !open {
			// return
		}
		fmt.Println("received new sequence notification via zmq", hex.EncodeToString(msg.Hash[:]), msg.Event)
	}
}
