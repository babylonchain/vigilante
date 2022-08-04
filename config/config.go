package config

import (
	"github.com/spf13/viper"
)

const (
	// DefaultGRPCAddress defines the default address to bind the gRPC server to.
	DefaultGRPCAddress = "0.0.0.0:8080"

	// DefaultGRPCWebAddress defines the default address to bind the gRPC-web server to.
	DefaultGRPCWebAddress = "0.0.0.0:8081"
)

// BaseConfig defines the server's basic configuration
type BaseConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

// APIConfig defines the API listener configuration.
type APIConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

// GRPCConfig defines configuration for the gRPC server.
type GRPCConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

// GRPCWebConfig defines configuration for the gRPC-web server.
type GRPCWebConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

// Config defines the server's top level configuration
type Config struct {
	BaseConfig `mapstructure:",squash"`

	API     APIConfig     `mapstructure:"api"`
	GRPC    GRPCConfig    `mapstructure:"grpc"`
	GRPCWeb GRPCWebConfig `mapstructure:"grpc-web"`
}

// DefaultConfig returns server's default configuration.
func DefaultConfig() *Config {
	return &Config{
		BaseConfig: BaseConfig{
			Placeholder: "baseconfig",
		},
		API: APIConfig{
			Placeholder: "apiconfig",
		},
		GRPC: GRPCConfig{
			Placeholder: "grpcconfig",
		},
		GRPCWeb: GRPCWebConfig{
			Placeholder: "grpcwebconfig",
		},
	}
}

// GetConfig returns a fully parsed Config object.
func GetConfig(v *viper.Viper) Config {
	return Config{
		BaseConfig: BaseConfig{
			Placeholder: v.GetString("placeholder"),
		},
		API: APIConfig{
			Placeholder: v.GetString("placeholder"),
		},
		GRPC: GRPCConfig{
			Placeholder: v.GetString("placeholder"),
		},
		GRPCWeb: GRPCWebConfig{
			Placeholder: v.GetString("placeholder"),
		},
	}
}
