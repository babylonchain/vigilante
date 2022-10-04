package config

import "time"

const (
	defaultRetrySleepTime    = "5s"
	defaultMaxRetrySleepTime = "5m"
)

// CommonConfig defines the server's basic configuration
type CommonConfig struct {
	// Backoff interval for the first retry.
	RetrySleepTime string `mapstructure:"retry-sleep-time"`

	// Maximum backoff interval between retries. Exponential backoff leads to interval increase.
	// This value is the cap of the interval, when exceeded the retries stop.
	MaxRetrySleepTime string `mapstructure:"max-retry-sleep-time"`
}

func (cfg *CommonConfig) Validate() error {
	if _, err := time.ParseDuration(cfg.RetrySleepTime); err != nil {
		return err
	}

	if _, err := time.ParseDuration(cfg.MaxRetrySleepTime); err != nil {
		return err
	}

	return nil
}

func DefaultCommonConfig() CommonConfig {
	return CommonConfig{
		RetrySleepTime:    defaultRetrySleepTime,
		MaxRetrySleepTime: defaultMaxRetrySleepTime,
	}
}
