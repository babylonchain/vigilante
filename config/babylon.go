package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/strangelove-ventures/lens/client"
)

// BabylonConfig defines configuration for the Babylon client
// adapted from https://github.com/strangelove-ventures/lens/blob/main/client/config.go
type BabylonConfig struct {
	Key            string                  `mapstructure:"key"`
	ChainID        string                  `mapstructure:"chain-id"`
	RPCAddr        string                  `mapstructure:"rpc-addr"`
	GRPCAddr       string                  `mapstructure:"grpc-addr"`
	AccountPrefix  string                  `mapstructure:"account-prefix"`
	KeyringBackend string                  `mapstructure:"keyring-backend"`
	GasAdjustment  float64                 `mapstructure:"gas-adjustment"`
	GasPrices      string                  `mapstructure:"gas-prices"`
	KeyDirectory   string                  `mapstructure:"key-directory"`
	Debug          bool                    `mapstructure:"debug"`
	Timeout        string                  `mapstructure:"timeout"`
	BlockTimeout   string                  `mapstructure:"block-timeout"`
	OutputFormat   string                  `mapstructure:"output-format"`
	SignModeStr    string                  `mapstructure:"sign-mode"`
	Modules        []module.AppModuleBasic `mapstructure:"-"`
}

func (cfg *BabylonConfig) Validate() error {
	if _, err := time.ParseDuration(cfg.Timeout); err != nil {
		return err
	}
	if cfg.BlockTimeout != "" {
		if _, err := time.ParseDuration(cfg.BlockTimeout); err != nil {
			return err
		}
	}
	return nil
}

func (cfg *BabylonConfig) Unwrap() *client.ChainClientConfig {
	return &client.ChainClientConfig{
		Key:            cfg.Key,
		ChainID:        cfg.ChainID,
		RPCAddr:        cfg.RPCAddr,
		GRPCAddr:       cfg.GRPCAddr,
		AccountPrefix:  cfg.AccountPrefix,
		KeyringBackend: cfg.KeyringBackend,
		GasAdjustment:  cfg.GasAdjustment,
		GasPrices:      cfg.GasPrices,
		KeyDirectory:   cfg.KeyDirectory,
		Debug:          cfg.Debug,
		Timeout:        cfg.Timeout,
		OutputFormat:   cfg.OutputFormat,
		SignModeStr:    cfg.SignModeStr,
	}
}

func DefaultBabylonConfig() BabylonConfig {
	return BabylonConfig{
		Key:            "default",
		ChainID:        "babylon-test",
		RPCAddr:        "https://localhost:1317", // see https://docs.cosmos.network/master/core/grpc_rest.html for default ports
		GRPCAddr:       "https://localhost:9090",
		AccountPrefix:  "babylon",
		KeyringBackend: "test",
		GasAdjustment:  1.2,
		GasPrices:      "0.01uatom",
		KeyDirectory:   defaultBabylonHome(),
		Debug:          true,
		Timeout:        "20s",
		OutputFormat:   "json",
		SignModeStr:    "direct",
	}
}

// defaultBabylonHome returns the default Babylon node directory, which is $HOME/.babylond
// copied from https://github.com/babylonchain/babylon/blob/main/app/app.go#L205-L210
func defaultBabylonHome() string {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return filepath.Join(userHomeDir, ".babylond")
}
