package config

// BaseConfig defines the server's basic configuration
type BaseConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

func (cfg *BaseConfig) Validate() error {
	return nil
}

func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Placeholder: "baseconfig",
	}
}
