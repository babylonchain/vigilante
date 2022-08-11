package bblclient

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/strangelove-ventures/lens/client"
)

// Client represents a persistent client connection to a bitcoin RPC server
// for information regarding the current best block chain.
type Client struct {
	*client.ChainClient
	Cfg *config.BabylonConfig
}
