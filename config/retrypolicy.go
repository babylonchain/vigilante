package config

import "time"

const (
	defaultInitialInterval = "1s"
	defaultMaxInterval     = "1m"
)

// RetryPolicyConfig defines the retry policy
type RetryPolicyConfig struct {
	// Backoff interval for the first retry.
	InitialInterval string `mapstructure:"initial-interval"`

	// Maximum backoff interval between retries. Exponential backoff leads to interval increase.
	// This value is the cap of the interval, when exceeded the retries stop.
	MaxInterval string `mapstructure:"max-interval"`
}

func (cfg *RetryPolicyConfig) Validate() error {
	if _, err := time.ParseDuration(cfg.InitialInterval); err != nil {
		return err
	}

	if _, err := time.ParseDuration(cfg.MaxInterval); err != nil {
		return err
	}

	return nil
}

func DefaultRetryPolicyConfig() RetryPolicyConfig {
	return RetryPolicyConfig{
		InitialInterval: defaultInitialInterval,
		MaxInterval:     defaultMaxInterval,
	}
}
