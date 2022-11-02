package btcclient

import (
	"fmt"
	"time"

	"github.com/babylonchain/babylon/types/retry"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

// NewWithBlockSubscriber creates a new BTC client that subscribes to newly connected/disconnected blocks
// used by vigilant reporter
func NewWithBlockSubscriber(cfg *config.BTCConfig, retrySleepTime, maxRetrySleepTime time.Duration) (*Client, error) {
	client := &Client{}
	params := netparams.GetBTCParams(cfg.NetParams)
	client.blockEventChan = make(chan *types.BlockEvent, 10000) // TODO: parameterise buffer size
	client.Cfg = cfg
	client.Params = params

	client.retrySleepTime = retrySleepTime
	client.maxRetrySleepTime = maxRetrySleepTime

	notificationHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			log.Debugf("Block %v at height %d has been connected at time %v", header.BlockHash(), height, header.Timestamp)
			client.blockEventChan <- types.NewBlockEvent(types.BlockConnected, height, header)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			log.Debugf("Block %v at height %d has been disconnected at time %v", header.BlockHash(), height, header.Timestamp)
			client.blockEventChan <- types.NewBlockEvent(types.BlockDisconnected, height, header)
		},
	}

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Endpoint,
		Endpoint:     "ws", // websocket
		User:         cfg.Username,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableClientTLS,
		Params:       params.Name,
		Certificates: readCAFile(cfg),
	}

	rpcClient, err := rpcclient.New(connCfg, &notificationHandlers)
	if err != nil {
		return nil, err
	}

	// ensure we are using btcd as Bitcoin node, since Websocket-based subscriber is only available in btcd
	backend, err := rpcClient.BackendVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get BTC backend: %v", err)
	}
	if backend != rpcclient.Btcd {
		return nil, fmt.Errorf("NewWithBlockSubscriber is only compatible with Btcd")
	}

	client.Client = rpcClient
	log.Info("Successfully created the BTC client and connected to the BTC server")

	return client, nil
}

func (c *Client) subscribeBlocksByWebSocket() error {
	if err := c.NotifyBlocks(); err != nil {
		return err
	}
	log.Info("Successfully subscribed to newly connected/disconnected blocks via WebSocket")
	return nil
}

func (c *Client) mustSubscribeBlocksByWebSocket() {
	if err := retry.Do(c.retrySleepTime, c.maxRetrySleepTime, func() error {
		return c.subscribeBlocksByWebSocket()
	}); err != nil {
		panic(err)
	}
}

func (c *Client) MustSubscribeBlocks() {
	// TODO: implement ZMQ-based block subscription
	c.mustSubscribeBlocksByWebSocket()
}

func (c *Client) BlockEventChan() <-chan *types.BlockEvent {
	return c.blockEventChan
}
