package config

import (
	"fmt"
)

const (
	defaultCheckpointBufferSize         = 100
	defaultBtcBlockBufferSize           = 100
	defaultBtcCacheSize                 = 100
	defaultLivenessCheckIntervalSeconds = 10
	defaultMaxLiveBtcHeights            = 100
)

// MonitorConfig defines the Monitor's basic configuration
type MonitorConfig struct {
	// Max number of checkpoints in the buffer
	CheckpointBufferSize uint64 `mapstructure:"checkpoint-buffer-size"`
	// Max number of BTC blocks in the buffer
	BtcBlockBufferSize uint64 `mapstructure:"btc-block-buffer-size"`
	// Max number of BTC blocks in the cache
	BtcCacheSize uint64 `mapstructure:"btc-cache-size"`

	LivenessCheckIntervalSeconds uint64 `mapstructure:"liveness-check-interval-seconds"`
	// Max lasting BTC heights that a checkpoint is not reported before an alarm is sent
	MaxLiveBtcHeights uint64 `mapstructure:"max-live-btc-heights"`
}

func (cfg *MonitorConfig) Validate() error {
	if cfg.CheckpointBufferSize < defaultCheckpointBufferSize {
		return fmt.Errorf("checkpoint-buffer-size should not be less than %v", defaultCheckpointBufferSize)
	}
	if cfg.BtcCacheSize < defaultBtcCacheSize {
		return fmt.Errorf("btc-cache-size should not be less than %v", defaultCheckpointBufferSize)
	}
	return nil
}

func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		CheckpointBufferSize:         defaultCheckpointBufferSize,
		BtcBlockBufferSize:           defaultBtcBlockBufferSize,
		BtcCacheSize:                 defaultBtcCacheSize,
		LivenessCheckIntervalSeconds: defaultLivenessCheckIntervalSeconds,
		MaxLiveBtcHeights:            defaultMaxLiveBtcHeights,
	}
}
