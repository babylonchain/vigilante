package config

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams string `mapstructure:"netparams"` // should be mainnet|testnet|simnet
}

func (cfg *ReporterConfig) Validate() error {
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams: "simnet",
	}
}
