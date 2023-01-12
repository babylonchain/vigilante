package config

import (
	"fmt"
)

const (
	defaultCheckpointBufferSize = 100
	defaultBtcBlockBufferSize   = 100
)

// MonitorConfig defines the Monitor's basic configuration
type MonitorConfig struct {
	// Max number of checkpoints in the buffer
	CheckpointBufferSize uint64 `mapstructure:"checkpoint-buffer-size"`
	// Max number of BTC blocks in the buffer
	BtcBlockBufferSize uint64 `mapstructure:"btc-block-buffer-size"`
}

func (cfg *MonitorConfig) Validate() error {
	if cfg.CheckpointBufferSize < defaultCheckpointBufferSize {
		return fmt.Errorf("checkpoint buffer size should not be less than %v", defaultCheckpointBufferSize)
	}
	return nil
}

func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		CheckpointBufferSize: defaultCheckpointBufferSize,
		BtcBlockBufferSize:   defaultBtcBlockBufferSize,
	}
}
