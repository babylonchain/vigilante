package config

import (
	"fmt"
)

const (
	defaultMetricServerPort = 2112
)

// MetricConfig defines the server's basic configuration
type MetricConfig struct {
	MetricServerPort int `mapstructure:"metric-server-port"`
}

func (cfg *MetricConfig) Validate() error {
	if cfg.MetricServerPort < 0 || cfg.MetricServerPort > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.MetricServerPort)
	}
	return nil
}

func DefaultMetricConfig() MetricConfig {
	return MetricConfig{
		MetricServerPort: defaultMetricServerPort,
	}
}
