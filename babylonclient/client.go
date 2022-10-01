package babylonclient

import (
	"fmt"

	"github.com/babylonchain/vigilante/config"
	lensclient "github.com/strangelove-ventures/lens/client"
)

var _ BabylonClient = &Client{}

type Client struct {
	*lensclient.ChainClient
	Cfg         *config.BabylonConfig
	RetryPolicy *config.RetryPolicyConfig
}

func New(cfg *config.BabylonConfig, retryPolicy *config.RetryPolicyConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if err := retryPolicy.Validate(); err != nil {
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
	client := &Client{cc, cfg, retryPolicy}
	log.Infof("Successfully created the Babylon client")

	return client, nil
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
