package config

import (
	"fmt"

	"github.com/babylonchain/vigilante/types"
)

const (
	minBTCCacheSize = 1000
	maxHeadersInMsg = 100 // maximum number of headers in a MsgInsertHeaders message
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams       string `mapstructure:"netparams"`          // should be mainnet|testnet|simnet|signet
	BTCCacheSize    uint64 `mapstructure:"btc_cache_size"`     // size of the BTC cache
	MaxHeadersInMsg uint32 `mapstructure:"max_headers_in_msg"` // maximum number of headers in a MsgInsertHeaders message
}

func (cfg *ReporterConfig) Validate() error {
	if _, ok := types.GetValidNetParams()[cfg.NetParams]; !ok {
		return fmt.Errorf("invalid net params")
	}
	if cfg.BTCCacheSize < minBTCCacheSize {
		return fmt.Errorf("BTC cache size has to be at least %d", minBTCCacheSize)
	}
	if cfg.MaxHeadersInMsg < maxHeadersInMsg {
		return fmt.Errorf("max_headers_in_msg has to be at least %d", maxHeadersInMsg)
	}
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams:       types.BtcSimnet.String(),
		BTCCacheSize:    minBTCCacheSize,
		MaxHeadersInMsg: maxHeadersInMsg,
	}
}
