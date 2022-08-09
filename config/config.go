package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/spf13/viper"
)

const (
	// DefaultGRPCAddress defines the default address to bind the gRPC server to.
	DefaultGRPCAddress = "0.0.0.0:8080"

	// DefaultGRPCWebAddress defines the default address to bind the gRPC-web server to.
	DefaultGRPCWebAddress = "0.0.0.0:8081"

	defaultConfigFilename   = "vigilante.yaml"
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
	DisableClientTLS  bool   `mapstructure:"noclienttls"`
	CAFile            string `mapstructure:"cafile"`
	Endpoint          string `mapstructure:"endpoint"`
	NetParams         string `mapstructure:"netparams"`
	Username          string `mapstructure:"username"`
	Password          string `mapstructure:"password"`
	ReconnectAttempts int    `mapstructure:"reconnect"`
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

// SubmitterConfig defines configuration for the gRPC-web server.
type SubmitterConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

// ReporterConfig defines configuration for the gRPC-web server.
type ReporterConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

// Config defines the server's top level configuration
type Config struct {
	Base      BaseConfig      `mapstructure:"base"`
	BTC       BTCConfig       `mapstructure:"btc"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	GRPCWeb   GRPCWebConfig   `mapstructure:"grpc-web"`
	Submitter SubmitterConfig `mapstructure:"submitter"`
	Reporter  ReporterConfig  `mapstructure:"reporter"`
}

// DefaultConfig returns server's default configuration.
func DefaultConfig() *Config {
	return &Config{
		Base: BaseConfig{
			Placeholder: "baseconfig",
		},
		BTC: BTCConfig{
			DisableClientTLS:  false,
			CAFile:            btcdDefaultCAFile,
			Endpoint:          "localhost:18554",
			NetParams:         "simnet",
			Username:          "user",
			Password:          "pass",
			ReconnectAttempts: 3,
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
		Submitter: SubmitterConfig{
			Placeholder: "submitterconfig",
		},
		Reporter: ReporterConfig{
			Placeholder: "reporterconfig",
		},
	}
}

// New returns a fully parsed Config object, from either
// - the config file in the default directory, or
// - the default config object (if the config file in the default directory does not exist)
func New() (Config, error) {
	if _, err := os.Stat(defaultConfigFile); err == nil { // read config from default config file
		viper.SetConfigFile(defaultConfigFile)
		if err := viper.ReadInConfig(); err != nil {
			return Config{}, err
		}
		log.Infof("successfully loaded config file at %s", defaultConfigFile)
		var cfg Config
		err = viper.Unmarshal(&cfg)
		return cfg, err
	} else if errors.Is(err, os.ErrNotExist) { // default config file does not exist, use the default config
		log.Infof("no config file found at %s, using the default config", defaultConfigFile)
		cfg := DefaultConfig()
		return *cfg, nil
	} else { // other errors
		return Config{}, err
	}
}

// NewFromFile returns a fully parsed Config object from a given file directory
func NewFromFile(configFile string) (Config, error) {
	if _, err := os.Stat(configFile); err == nil { // the given file exists, parse it
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			return Config{}, err
		}
		log.Infof("successfully loaded config file at %s", configFile)
		var cfg Config
		err = viper.Unmarshal(&cfg)
		return cfg, err
	} else if errors.Is(err, os.ErrNotExist) { // the given config file does not exist, return error
		return Config{}, fmt.Errorf("no config file found at %s", configFile)
	} else { // other errors
		return Config{}, err
	}
}
