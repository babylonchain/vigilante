package config

// BaseConfig defines the server's basic configuration
type BaseConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Placeholder: "baseconfig",
	}
}
