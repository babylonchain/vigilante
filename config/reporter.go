package config

import (
	"fmt"

	"github.com/babylonchain/vigilante/types"
)

const (
	minBTCCacheSize = 1000
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams    string `mapstructure:"netparams"`      // should be mainnet|testnet|simnet|signet
	BTCCacheSize uint64 `mapstructure:"btc_cache_size"` // size of the BTC cache
}

func (cfg *ReporterConfig) Validate() error {
	if _, ok := types.GetValidNetParams()[cfg.NetParams]; !ok {
		return fmt.Errorf("invalid net params")
	}
	if cfg.BTCCacheSize < minBTCCacheSize {
		return fmt.Errorf("BTC cache size has to be at least %d", minBTCCacheSize)
	}
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams:    types.BtcSimnet.String(),
		BTCCacheSize: minBTCCacheSize,
	}
}
