package config

import (
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
)

const (
	DefaultCheckpointCacheMaxEntries = 100
	DefaultPollingIntervalSeconds    = 60 // in seconds
)

// SubmitterConfig defines configuration for the gRPC-web server.
type SubmitterConfig struct {
	NetParams              string `mapstructure:"netparams"`   // should be mainnet|testnet|simnet
	BufferSize             uint   `mapstructure:"buffer-size"` // buffer for raw checkpoints
	PollingIntervalSeconds uint   `mapstructure:"polling-interval-seconds"`
}

func (cfg *SubmitterConfig) Validate() error {
	return nil
}

func (cfg *SubmitterConfig) GetTag() btctxformatter.BabylonTag {
	return netparams.GetBabylonParams(cfg.NetParams).Tag
}

func (cfg *SubmitterConfig) GetVersion() btctxformatter.FormatVersion {
	return netparams.GetBabylonParams(cfg.NetParams).Version
}

func DefaultSubmitterConfig() SubmitterConfig {
	return SubmitterConfig{
		NetParams:              "simnet",
		BufferSize:             DefaultCheckpointCacheMaxEntries,
		PollingIntervalSeconds: DefaultPollingIntervalSeconds,
	}
}
