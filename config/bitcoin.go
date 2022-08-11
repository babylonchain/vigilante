package config

// BTCConfig defines configuration for the Bitcoin client
type BTCConfig struct {
	DisableClientTLS  bool   `mapstructure:"noclienttls"`
	CAFile            string `mapstructure:"cafile"`
	Endpoint          string `mapstructure:"endpoint"`
	NetParams         string `mapstructure:"netparams"`
	Username          string `mapstructure:"username"`
	Password          string `mapstructure:"password"`
	ReconnectAttempts int    `mapstructure:"reconnect"`
}

func DefaultBTCConfig() BTCConfig {
	return BTCConfig{
		DisableClientTLS:  false,
		CAFile:            btcdDefaultCAFile,
		Endpoint:          "localhost:18554",
		NetParams:         "simnet",
		Username:          "user",
		Password:          "pass",
		ReconnectAttempts: 3,
	}
}
