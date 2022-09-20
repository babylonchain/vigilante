package config

import "time"

const (
	defaultRetrySleepTime    = "5s"
	defaultMaxRetrySleepTime = "5m"
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams         string `mapstructure:"netparams"` // should be mainnet|testnet|simnet
	RetrySleepTime    string `mapstructure:"retry-sleep-time"`
	MaxRetrySleepTime string `mapstructure:"max-retry-sleep-time"`
}

func (cfg *ReporterConfig) Validate() error {
	if _, err := time.ParseDuration(cfg.RetrySleepTime); err != nil {
		return err
	}

	if _, err := time.ParseDuration(cfg.MaxRetrySleepTime); err != nil {
		return err
	}

	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams:         "simnet",
		RetrySleepTime:    defaultRetrySleepTime,
		MaxRetrySleepTime: defaultMaxRetrySleepTime,
	}
}
