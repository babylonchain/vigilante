package config

const (
	// DefaultGRPCAddress defines the default address to bind the gRPC server to.
	DefaultGRPCAddress = "0.0.0.0:8080"
)

// GRPCConfig defines configuration for the gRPC server.
type GRPCConfig struct {
	OneTimeTLSKey bool     `mapstructure:"onetimetlskey"`
	RPCKeyFile    string   `mapstructure:"rpckey"`
	RPCCertFile   string   `mapstructure:"rpccert"`
	Endpoints     []string `mapstructure:"endpoints"`
}

func (cfg *GRPCConfig) Validate() error {
	return nil
}

func DefaultGRPCConfig() GRPCConfig {
	return GRPCConfig{
		OneTimeTLSKey: true,
		RPCKeyFile:    defaultRPCKeyFile,
		RPCCertFile:   defaultRPCCertFile,
		Endpoints:     []string{"localhost:8080"},
	}
}
