package config

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"

	"github.com/babylonchain/vigilante/types"
)

// BTCConfig defines configuration for the Bitcoin client
type BTCConfig struct {
	DisableClientTLS  bool                      `mapstructure:"no-client-tls"`
	CAFile            string                    `mapstructure:"ca-file"`
	Endpoint          string                    `mapstructure:"endpoint"`
	WalletEndpoint    string                    `mapstructure:"wallet-endpoint"`
	WalletPassword    string                    `mapstructure:"wallet-password"`
	WalletName        string                    `mapstructure:"wallet-name"`
	WalletCAFile      string                    `mapstructure:"wallet-ca-file"`
	WalletLockTime    int64                     `mapstructure:"wallet-lock-time"` // time duration in which the wallet remains unlocked, in seconds
	TxRelayFeeMin     btcutil.Amount            `mapstructure:"tx-relay-fee-min"` // minimum fee for a tx to be relayed by Bitcoin
	TxFeeMin          btcutil.Amount            `mapstructure:"tx-fee-min"`       // minimum tx fee, sat/byte
	TxFeeMax          btcutil.Amount            `mapstructure:"tx-fee-max"`       // maximum tx fee, sat/byte
	TargetBlockNum    int64                     `mapstructure:"target-block-num"` // this implies how soon the tx is estimated to be included in a block, e.g., 1 means the tx is estimated to be included in the next block
	NetParams         string                    `mapstructure:"net-params"`
	Username          string                    `mapstructure:"username"`
	Password          string                    `mapstructure:"password"`
	ReconnectAttempts int                       `mapstructure:"reconnect-attempts"`
	BtcBackend        types.SupportedBtcBackend `mapstructure:"btc-backend"`
	ZmqEndpoint       string                    `mapstructure:"zmq-endpoint"`
}

func (cfg *BTCConfig) Validate() error {
	if cfg.ReconnectAttempts < 0 {
		return errors.New("reconnect-attempts must be non-negative")
	}

	if _, ok := types.GetValidNetParams()[cfg.NetParams]; !ok {
		return errors.New("invalid net params")
	}

	if _, ok := types.GetValidBtcBackends()[cfg.BtcBackend]; !ok {
		return errors.New("invalid btc backend")
	}

	if cfg.BtcBackend == types.Bitcoind {
		// TODO: implement regex validation for zmq endpoint
		if cfg.ZmqEndpoint == "" {
			return errors.New("zmq endpoint cannot be empty")
		}
	}

	if cfg.TargetBlockNum <= 0 {
		return errors.New("target-block-num should be positive")
	}

	if cfg.TxFeeMin == 0 {
		return errors.New("tx-fee-min should not be zero")
	}

	if cfg.TxRelayFeeMin == 0 {
		return errors.New("tx-relay-fee-min should not be zero")
	}

	if cfg.TxFeeMax > btcutil.MaxSatoshi {
		return fmt.Errorf("tx-fee-max should not be larger than %v", btcutil.MaxSatoshi)
	}

	if cfg.TxFeeMin > cfg.TxFeeMax {
		return errors.New("tx-fee-min should not be larger than tx-fee-max")
	}

	if cfg.TxRelayFeeMin > cfg.TxFeeMin {
		return errors.New("tx-relay-fee-min should not be larger than tx-fee-min")
	}

	if cfg.TxRelayFeeMin > cfg.TxFeeMax {
		return errors.New("tx-relay-fee-min should not be larger than tx-fee-max")
	}

	return nil
}

func DefaultBTCConfig() BTCConfig {

	return BTCConfig{
		DisableClientTLS:  false,
		CAFile:            defaultBtcCAFile,
		Endpoint:          "localhost:18556",
		WalletEndpoint:    "localhost:18554",
		WalletPassword:    "walletpass",
		WalletName:        "default",
		WalletCAFile:      defaultBtcWalletCAFile,
		WalletLockTime:    10,
		TxRelayFeeMin:     btcutil.Amount(1),  // minimum tx relay fee, sat/byte
		TxFeeMin:          btcutil.Amount(1),  // minimum tx fee, sat/byte
		TxFeeMax:          btcutil.Amount(20), // maximum tx fee, sat/byte
		TargetBlockNum:    1,
		NetParams:         types.BtcSimnet.String(),
		Username:          "rpcuser",
		Password:          "rpcpass",
		ReconnectAttempts: 3,
	}
}
