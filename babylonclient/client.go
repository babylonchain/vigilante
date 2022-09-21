package babylonclient

import (
	"context"
	"fmt"
	"github.com/babylonchain/vigilante/config"
	lensclient "github.com/strangelove-ventures/lens/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

type Client struct {
	*lensclient.ChainClient
	Cfg *config.BabylonConfig
	eCh <-chan ctypes.ResultEvent
}

func New(cfg *config.BabylonConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// create a Tendermint/Cosmos client for Babylon
	cc, err := newLensClient(cfg.Unwrap())
	if err != nil {
		return nil, err
	}

	// show addresses in the key ring
	// TODO: specify multiple addresses in config
	addrs, err := cc.ListAddresses()
	if err != nil {
		return nil, err
	}
	log.Debugf("Babylon key directory: %v", cfg.KeyDirectory)
	log.Debugf("All Babylon addresses: %v", addrs)

	// wrap to our type
	client := &Client{
		ChainClient: cc,
		Cfg:         cfg,
	}
	log.Infof("Successfully created the Babylon client")

	return client, nil
}

func NewWithSubscriber(cfg *config.BabylonConfig) (*Client, error) {
	bbnClient, err := New(cfg)
	if !bbnClient.RPCClient.IsRunning() {
		err = bbnClient.RPCClient.Start()
		if err != nil {
			return nil, err
		}
	}
	eventsChan, err := bbnClient.RPCClient.Subscribe(context.Background(), bbnClient.Config.ChainID,
		types.QueryForEvent(types.EventTx).String())
	if err != nil {
		return nil, err
	}
	bbnClient.eCh = eventsChan

	log.Infof("Successfully created the Babylon client that subscribes events from Babylon")

	return bbnClient, nil
}

func (c Client) GetEvent() <-chan ctypes.ResultEvent {
	return c.eCh
}

func (c Client) GetTagIdx() uint8 {
	tagIdxStr := c.Cfg.TagIdx
	if len(tagIdxStr) != 1 {
		panic(fmt.Errorf("tag index should be one byte"))
	}
	// convert tagIdx from string to its ascii value
	return uint8(rune(tagIdxStr[0]))
}

func (c *Client) Stop() {
	if c.RPCClient != nil && c.RPCClient.IsRunning() {
		<-c.RPCClient.Quit()
	}
}
