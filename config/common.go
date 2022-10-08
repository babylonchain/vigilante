package config

import (
	"errors"
	"time"
)

const (
	defaultRetrySleepTime    = 5 * time.Second
	defaultMaxRetrySleepTime = 5 * time.Minute
)

// CommonConfig defines the server's basic configuration
type CommonConfig struct {
	// Backoff interval for the first retry.
	RetrySleepTime time.Duration `mapstructure:"retry-sleep-time"`

	// Maximum backoff interval between retries. Exponential backoff leads to interval increase.
	// This value is the cap of the interval, when exceeded the retries stop.
	MaxRetrySleepTime time.Duration `mapstructure:"max-retry-sleep-time"`
}

func (cfg *CommonConfig) Validate() error {

	if cfg.RetrySleepTime < 0 {
		return errors.New("retry-sleep-time can't be negative")
	}
	if cfg.MaxRetrySleepTime < 0 {
		return errors.New("max-retry-sleep-time can't be negative")
	}
	return nil
}

func DefaultCommonConfig() CommonConfig {
	return CommonConfig{
		RetrySleepTime:    defaultRetrySleepTime,
		MaxRetrySleepTime: defaultMaxRetrySleepTime,
	}
}
