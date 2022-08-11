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

// Config defines the server's top level configuration
type Config struct {
	Base      BaseConfig      `mapstructure:"base"`
	BTC       BTCConfig       `mapstructure:"btc"`
	Babylon   BabylonConfig   `mapstructure:"bbl"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	GRPCWeb   GRPCWebConfig   `mapstructure:"grpc-web"`
	Submitter SubmitterConfig `mapstructure:"submitter"`
	Reporter  ReporterConfig  `mapstructure:"reporter"`
}

// DefaultConfig returns server's default configuration.
func DefaultConfig() *Config {
	return &Config{
		Base:      DefaultBaseConfig(),
		BTC:       DefaultBTCConfig(),
		Babylon:   DefaultBabylonConfig(),
		GRPC:      DefaultGRPCConfig(),
		GRPCWeb:   DefaultGRPCWebConfig(),
		Submitter: DefaultSubmitterConfig(),
		Reporter:  DefaultReporterConfig(),
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
		log.Infof("Successfully loaded config file at %s", defaultConfigFile)
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
		log.Infof("Successfully loaded config file at %s", configFile)
		var cfg Config
		err = viper.Unmarshal(&cfg)
		return cfg, err
	} else if errors.Is(err, os.ErrNotExist) { // the given config file does not exist, return error
		return Config{}, fmt.Errorf("no config file found at %s", configFile)
	} else { // other errors
		return Config{}, err
	}
}
