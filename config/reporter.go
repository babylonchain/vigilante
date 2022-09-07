package config

const (
	DefaultBTCCacheMaxEntries = 1000
)

// ReporterConfig defines configuration for the reporter.
type ReporterConfig struct {
	NetParams          string `mapstructure:"netparams"` // should be mainnet|testnet|simnet
	BTCCacheMaxEntries uint   `mapstructure:"btc-cache-max-entries"`
}

func (cfg *ReporterConfig) Validate() error {
	return nil
}

func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		NetParams:          "simnet",
		BTCCacheMaxEntries: DefaultBTCCacheMaxEntries,
	}
}
