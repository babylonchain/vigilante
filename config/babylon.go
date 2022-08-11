package config

// BabylonConfig defines configuration for the Babylon client
type BabylonConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

func DefaultBabylonConfig() BabylonConfig {
	return BabylonConfig{
		Placeholder: "BabylonConfig",
	}
}
