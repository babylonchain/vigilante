package config

import "time"

const (
	defaultSleepTimeSeconds = 1
	defaultRetryAttempts    = 3
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams             string        `mapstructure:"netparams"` // should be mainnet|testnet|simnet
	RetrySleepTimeSeconds time.Duration `mapstructure:"retry-sleep-time-seconds"`
	RetryAttempts         int           `mapstructure:"retry-attempts"`
}

func (cfg *ReporterConfig) Validate() error {
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams:             "simnet",
		RetrySleepTimeSeconds: defaultSleepTimeSeconds,
		RetryAttempts:         defaultRetryAttempts,
	}
}
