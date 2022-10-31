package config

import (
	"errors"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"
)

// BTCConfig defines configuration for the Bitcoin client
type BTCConfig struct {
	DisableClientTLS     bool                            `mapstructure:"no-client-tls"`
	CAFile               string                          `mapstructure:"ca-file"`
	Endpoint             string                          `mapstructure:"endpoint"`
	WalletEndpoint       string                          `mapstructure:"wallet-endpoint"`
	WalletPassword       string                          `mapstructure:"wallet-password"`
	WalletName           string                          `mapstructure:"wallet-name"`
	WalletCAFile         string                          `mapstructure:"wallet-ca-file"`
	WalletLockTime       int64                           `mapstructure:"wallet-lock-time"` // time duration in which the wallet remains unlocked, in seconds
	TxFee                btcutil.Amount                  `mapstructure:"tx-fee"`           // BTC tx fee, in BTC
	NetParams            string                          `mapstructure:"net-params"`
	Username             string                          `mapstructure:"username"`
	Password             string                          `mapstructure:"password"`
	ReconnectAttempts    int                             `mapstructure:"reconnect-attempts"`
	SubscriptionMode     types.SupportedSubscriptionMode `mapstructure:"subscription-mode"`
	ZmqEndpoint          string                          `mapstructure:"zmq-endpoint"`
	ZmqChannelBufferSize int                             `mapstructure:"zmq-channel-buffer-size"`
}

func (cfg *BTCConfig) Validate() error {
	if cfg.ReconnectAttempts < 0 {
		return errors.New("reconnect-attempts must be non-negative")
	}

	if _, ok := types.GetValidNetParams()[cfg.NetParams]; !ok {
		return errors.New("invalid net params")
	}

	if _, ok := types.GetValidSubscriptionModes()[cfg.SubscriptionMode]; !ok {
		return errors.New("invalid subscription mode")
	}

	if cfg.SubscriptionMode == types.ZmqMode {
		// TODO: implement regex validation for zmq endpoint
		if cfg.ZmqEndpoint == "" {
			return errors.New("zmq endpoint cannot be empty")
		}

		if cfg.ZmqChannelBufferSize <= 0 {
			return errors.New("ZMQ channel buffer size must be positive")
		}
	}

	return nil
}

func DefaultBTCConfig() BTCConfig {
	feeAmount, err := btcutil.NewAmount(0.00001)
	if err != nil {
		panic(err)
	}
	return BTCConfig{
		DisableClientTLS:  false,
		CAFile:            defaultBtcCAFile,
		Endpoint:          "localhost:18556",
		WalletEndpoint:    "localhost:18554",
		WalletPassword:    "walletpass",
		WalletName:        "default",
		WalletCAFile:      defaultBtcWalletCAFile,
		WalletLockTime:    10,
		TxFee:             feeAmount,
		NetParams:         types.BtcSimnet.String(),
		Username:          "rpcuser",
		Password:          "rpcpass",
		ReconnectAttempts: 3,
	}
}
