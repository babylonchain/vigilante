package config

import (
	"path/filepath"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/spf13/viper"
)

const (
	// DefaultGRPCAddress defines the default address to bind the gRPC server to.
	DefaultGRPCAddress = "0.0.0.0:8080"

	// DefaultGRPCWebAddress defines the default address to bind the gRPC-web server to.
	DefaultGRPCWebAddress = "0.0.0.0:8081"

	defaultConfigFilename   = "vigilante.conf"
	defaultLogLevel         = "info"
	defaultLogDirname       = "logs"
	defaultLogFilename      = "vigilante.log"
	defaultRPCMaxClients    = 10
	defaultRPCMaxWebsockets = 25
)

var (
	btcdDefaultCAFile  = filepath.Join(btcutil.AppDataDir("btcd", false), "rpc.cert")
	defaultAppDataDir  = btcutil.AppDataDir("babylon-vigilante", false)
	defaultConfigFile  = filepath.Join(defaultAppDataDir, defaultConfigFilename)
	defaultRPCKeyFile  = filepath.Join(defaultAppDataDir, "rpc.key")
	defaultRPCCertFile = filepath.Join(defaultAppDataDir, "rpc.cert")
	defaultLogDir      = filepath.Join(defaultAppDataDir, defaultLogDirname)
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
	OneTimeTLSKey bool     `mapstructure:"onetimetlskey"`
	RPCKeyFile    string   `mapstructure:"rpckey"`
	RPCCertFile   string   `mapstructure:"rpccert"`
	Endpoints     []string `mapstructure:"endpoints"`
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
			OneTimeTLSKey: true,
			RPCKeyFile:    defaultRPCKeyFile,
			RPCCertFile:   defaultRPCCertFile,
			Endpoints:     []string{"localhost:8080"},
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
			OneTimeTLSKey: v.GetBool("onetimetlskey"),
			RPCKeyFile:    v.GetString("rpckey"),
			RPCCertFile:   v.GetString("rpccert"),
			Endpoints:     v.GetStringSlice("endpoints"),
		},
		GRPCWeb: GRPCWebConfig{
			Placeholder: v.GetString("placeholder"),
		},
	}
}
