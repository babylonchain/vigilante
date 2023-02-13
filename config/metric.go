package config

import (
	"fmt"
)

const (
	defaultMetricsServerPort = 2112
)

// MetricsConfig defines the server's basic configuration
type MetricsConfig struct {
	ServerPort int `mapstructure:"server-port"`
}

func (cfg *MetricsConfig) Validate() error {
	if cfg.ServerPort < 0 || cfg.ServerPort > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.ServerPort)
	}
	return nil
}

func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		ServerPort: defaultMetricsServerPort,
	}
}
