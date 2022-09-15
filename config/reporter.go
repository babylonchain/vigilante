package config

import "time"

const (
	defaultSleepInterval = 1
	defaultRetryAttempts = 3
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams          string        `mapstructure:"netparams"` // should be mainnet|testnet|simnet
	RetrySleepInterval time.Duration `mapstructure:"sleep-time"`
	RetryAttempts      int           `mapstructure:"retry-attempts"`
}

func (cfg *ReporterConfig) Validate() error {
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams:          "simnet",
		RetrySleepInterval: defaultSleepInterval,
		RetryAttempts:      defaultRetryAttempts,
	}
}
