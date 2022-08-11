package bblclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/strangelove-ventures/lens/client"
	"go.uber.org/zap"
)

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*client.ChainClient
	Cfg *config.BabylonConfig
}

func New(cfg *config.BabylonConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// init Zap logger, which is required by ChainClient
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	// create chainClient
	chainClient, err := client.NewChainClient(
		logger,
		cfg.Unwrap(),
		cfg.KeyDirectory,
		nil, // TODO: figure out this field
		nil, // TODO: figure out this field
	)
	if err != nil {
		return nil, err
	}
	// wrap to our type
	client := &Client{chainClient, cfg}

	return client, nil
}
