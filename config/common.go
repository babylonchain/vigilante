package config

import (
	"errors"
	"time"

	"go.uber.org/zap"
)

const (
	defaultRetrySleepTime    = 5 * time.Second
	defaultMaxRetrySleepTime = 5 * time.Minute
)

// CommonConfig defines the server's basic configuration
type CommonConfig struct {
	// LogFormat is the format of the log (json|auto|console|logfmt)
	LogFormat string `mapstructure:"log-format"`
	// LogLevel is the log level (debug|warn|error|panic|fatal)
	LogLevel string `mapstructure:"log-level"`
	// Backoff interval for the first retry.
	RetrySleepTime time.Duration `mapstructure:"retry-sleep-time"`
	// Maximum backoff interval between retries. Exponential backoff leads to interval increase.
	// This value is the cap of the interval, when exceeded the retries stop.
	MaxRetrySleepTime time.Duration `mapstructure:"max-retry-sleep-time"`
}

func isOneOf(v string, list []string) bool {
	for _, item := range list {
		if v == item {
			return true
		}
	}
	return false
}

func (cfg *CommonConfig) Validate() error {
	if !isOneOf(cfg.LogFormat, []string{"json", "auto", "console", "logfmt"}) {
		return errors.New("log-format is not one of json|auto|console|logfmt")
	}
	if !isOneOf(cfg.LogLevel, []string{"debug", "warn", "error", "panic", "fatal"}) {
		return errors.New("log-level is not one of debug|warn|error|panic|fatal")
	}
	if cfg.RetrySleepTime < 0 {
		return errors.New("retry-sleep-time can't be negative")
	}
	if cfg.MaxRetrySleepTime < 0 {
		return errors.New("max-retry-sleep-time can't be negative")
	}
	return nil
}

func (cfg *CommonConfig) CreateLogger() (*zap.Logger, error) {
	return NewRootLogger(cfg.LogFormat, cfg.LogLevel)
}

func DefaultCommonConfig() CommonConfig {
	return CommonConfig{
		LogFormat:         "auto",
		LogLevel:          "debug",
		RetrySleepTime:    defaultRetrySleepTime,
		MaxRetrySleepTime: defaultMaxRetrySleepTime,
	}
}
