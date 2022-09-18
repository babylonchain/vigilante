package config

import "time"

const (
	defaultRetryAttempts = 3
	defaultSleepTime     = "5s"
	defaultRetryTimeout  = "5m"
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams      string `mapstructure:"netparams"` // should be mainnet|testnet|simnet
	RetryAttempts  int    `mapstructure:"retry-attempts"`
	RetrySleepTime string `mapstructure:"retry-sleep-time"`
	RetryTimeout   string `mapstructure:"retry-timeout"`
}

func (cfg *ReporterConfig) Validate() error {
	if _, err := time.ParseDuration(cfg.RetrySleepTime); err != nil {
		return err
	}

	if _, err := time.ParseDuration(cfg.RetryTimeout); err != nil {
		return err
	}

	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams:      "simnet",
		RetryAttempts:  defaultRetryAttempts,
		RetrySleepTime: defaultSleepTime,
		RetryTimeout:   defaultRetryTimeout,
	}
}
