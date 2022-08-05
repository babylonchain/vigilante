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

// BTCConfig defines the server's basic configuration
type BTCConfig struct {
	DisableClientTLS bool   `mapstructure:"noclienttls"`
	CAFile           string `mapstructure:"cafile"`
	Endpoint         string `mapstructure:"endpoint"`
	NetParams        string `mapstructure:"netparams"`
	Username         string `mapstructure:"username"`
	Password         string `mapstructure:"password"`
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

	BTC     BTCConfig     `mapstructure:"btc"`
	GRPC    GRPCConfig    `mapstructure:"grpc"`
	GRPCWeb GRPCWebConfig `mapstructure:"grpc-web"`
}

// DefaultConfig returns server's default configuration.
func DefaultConfig() *Config {
	return &Config{
		BaseConfig: BaseConfig{
			Placeholder: "baseconfig",
		},
		BTC: BTCConfig{
			DisableClientTLS: true,
			Endpoint:         "localhost:18554",
			NetParams:        "simnet",
			Username:         "user",
			Password:         "pass",
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
		BTC: BTCConfig{
			DisableClientTLS: v.GetBool("noclienttls"),
			CAFile:           v.GetString("cafile"),
			Endpoint:         v.GetString("endpoint"),
			NetParams:        v.GetString("netparams"),
			Username:         v.GetString("username"),
			Password:         v.GetString("password"),
		},
		GRPC: GRPCConfig{
			Placeholder: v.GetString("placeholder"),
		},
		GRPCWeb: GRPCWebConfig{
			Placeholder: v.GetString("placeholder"),
		},
	}
}
