package config

const (
	// DefaultGRPCWebAddress defines the default address to bind the gRPC-web server to.
	DefaultGRPCWebAddress = "0.0.0.0:8081"
)

// GRPCWebConfig defines configuration for the gRPC-web server.
type GRPCWebConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

func (cfg *GRPCWebConfig) Validate() error {
	return nil
}

func DefaultGRPCWebConfig() GRPCWebConfig {
	return GRPCWebConfig{
		Placeholder: "grpcwebconfig",
	}
}
