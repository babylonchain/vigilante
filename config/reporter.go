package config

import (
	"errors"
	"github.com/babylonchain/vigilante/types"
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams string `mapstructure:"netparams"` // should be mainnet|testnet|simnet
}

func (cfg *ReporterConfig) Validate() error {
	if _, ok := types.GetValidNetParams()[cfg.NetParams]; !ok {
		return errors.New("invalid net params")
	}
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams: types.BtcSimnet.String(),
	}
}
