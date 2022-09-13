package config

import (
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/btcutil"
)

const (
	DefaultWalletLockTime            = 10 // seconds
	DefaultCheckpointCacheMaxEntries = 100
	DefaultTransactionFees           = 0.00001 // BTC
	DefaultWalletPass                = "walletpass"
	DefaultWalletName                = "default"
	DefaultSubmitterAddress          = "bbn1v6k7k9s8md3k29cu9runasstq5zaa0lpznk27w"
)

// SubmitterConfig defines configuration for the gRPC-web server.
type SubmitterConfig struct {
	NetParams        string         `mapstructure:"netparams"` // should be mainnet|testnet|simnet
	TxFee            btcutil.Amount `mapstructure:"txfee"`
	BufferSize       uint           `mapstructure:"buffer-size"`
	WalletPass       string         `mapstructure:"wallet-pass"`
	WalletLockTime   uint           `mapstructure:"wallet-lock-time"` // in seconds
	WalletName       string         `mapstructure:"wallet-name"`
	SubmitterAddress string         `mapstructure:"submitter-address"`
}

func (cfg *SubmitterConfig) Validate() error {
	return nil
}

func (cfg *SubmitterConfig) GetTag() btctxformatter.BabylonTag {
	return netparams.GetBabylonParams(cfg.NetParams).Tag
}

func (cfg *SubmitterConfig) GetVersion() btctxformatter.FormatVersion {
	return netparams.GetBabylonParams(cfg.NetParams).Version
}

func DefaultSubmitterConfig() SubmitterConfig {
	amount, err := btcutil.NewAmount(DefaultTransactionFees)
	if err != nil {
		panic(err)
	}
	return SubmitterConfig{
		NetParams:        "simnet",
		TxFee:            amount,
		BufferSize:       DefaultCheckpointCacheMaxEntries,
		WalletPass:       DefaultWalletPass,
		WalletLockTime:   DefaultWalletLockTime,
		WalletName:       DefaultWalletName,
		SubmitterAddress: DefaultSubmitterAddress,
	}
}
