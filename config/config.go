package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	defaultConfigFilename = "vigilante.yml"
	// TODO: configure logging
	defaultLogLevel         = "info"
	defaultLogDirname       = "logs"
	defaultLogFilename      = "vigilante.log"
	defaultRPCMaxClients    = 10
	defaultRPCMaxWebsockets = 25
)

var (
	defaultBtcCAFile   = filepath.Join(btcutil.AppDataDir("btcd", false), "rpc.cert")
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
	Babylon   BabylonConfig   `mapstructure:"babylon"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	GRPCWeb   GRPCWebConfig   `mapstructure:"grpc-web"`
	Submitter SubmitterConfig `mapstructure:"submitter"`
	Reporter  ReporterConfig  `mapstructure:"reporter"`
}

func (cfg *Config) Validate() error {
	if err := cfg.Base.Validate(); err != nil {
		return err
	} else if err := cfg.BTC.Validate(); err != nil {
		return err
	} else if err := cfg.Babylon.Validate(); err != nil {
		return err
	} else if err := cfg.GRPC.Validate(); err != nil {
		return err
	} else if err := cfg.GRPCWeb.Validate(); err != nil {
		return err
	} else if err := cfg.Submitter.Validate(); err != nil {
		return err
	} else if err := cfg.Reporter.Validate(); err != nil {
		return err
	}
	return nil
}

func DefaultConfigFile() string {
	return defaultConfigFile
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

// New returns a fully parsed Config object from a given file directory
func New(configFile string) (Config, error) {
	if _, err := os.Stat(configFile); err == nil { // the given file exists, parse it
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			return Config{}, err
		}
		log.Infof("Successfully loaded config file at %s", configFile)
		var cfg Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return Config{}, err
		}
		if err := cfg.Validate(); err != nil {
			return Config{}, err
		}
		// Set Babylon modules to ModuleBasics since the configuration file does not contain that value
		// hack: We should find a better place to add this universal config
		cfg.Babylon.Modules = ModuleBasics
		return cfg, err
	} else if errors.Is(err, os.ErrNotExist) { // the given config file does not exist, return error
		return Config{}, fmt.Errorf("no config file found at %s", configFile)
	} else { // other errors
		return Config{}, err
	}
}

func WriteSample() error {
	cfg := DefaultConfig()
	d, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	// write to file
	err = ioutil.WriteFile("./sample-vigilante.yml", d, 0644)
	if err != nil {
		return err
	}
	return nil
}
