package config

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

func (cfg *ReporterConfig) Validate() error {
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		Placeholder: "submitterconfig",
	}
}
