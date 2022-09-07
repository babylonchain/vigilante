package babylonclient

import (
	"context"
	"fmt"
	"github.com/babylonchain/vigilante/config"
	lensclient "github.com/strangelove-ventures/lens/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
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

	query := fmt.Sprintf("tm.event=EventCheckpointSealed")
	eventsChan, err := cc.RPCClient.Subscribe(context.Background(), cc.Config.ChainID, query)
	// TODO: is context necessary here?
	// ctx := client.Context{}.
	// 	WithClient(cc.RPCClient).
	// 	WithInterfaceRegistry(cc.Codec.InterfaceRegistry).
	// 	WithChainID(cc.Config.ChainID).
	// 	WithCodec(cc.Codec.Marshaler)

	// wrap to our type
	client := &Client{
		ChainClient: cc,
		Cfg:         cfg,
		eCh:         eventsChan,
	}
	log.Infof("Successfully created the Babylon client")

	return client, nil
}

func (c Client) GetEvent() <-chan ctypes.ResultEvent {
	return c.eCh
}

func (c *Client) Stop() {
	if c.RPCClient != nil && c.RPCClient.IsRunning() {
		<-c.RPCClient.Quit()
	}
}
