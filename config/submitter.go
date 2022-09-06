package config

// SubmitterConfig defines configuration for the gRPC-web server.
type SubmitterConfig struct {
	NetParams string `mapstructure:"netparams"` // should be mainnet|testnet|simnet
}

func (cfg *SubmitterConfig) Validate() error {
	return nil
}

func DefaultSubmitterConfig() SubmitterConfig {
	return SubmitterConfig{
		NetParams: "simnet",
	}
}
