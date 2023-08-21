package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/lightningnetwork/lnd/lnwallet/chainfee"

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
	TxFeeMin          chainfee.SatPerKVByte     `mapstructure:"tx-fee-min"`       // minimum tx fee, sat/kvb
	TxFeeMax          chainfee.SatPerKVByte     `mapstructure:"tx-fee-max"`       // maximum tx fee, sat/kvb
	DefaultFee        chainfee.SatPerKVByte     `mapstructure:"default-fee"`      // default BTC tx fee in case estimation fails, sat/kvb
	EstimateMode      string                    `mapstructure:"estimate-mode"`    // the BTC tx fee estimate mode, which is only used by bitcoind, must be either ECONOMICAL or CONSERVATIVE
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

		if cfg.EstimateMode != "ECONOMICAL" && cfg.EstimateMode != "CONSERVATIVE" {
			return errors.New("estimate-mode must be either ECONOMICAL or CONSERVATIVE when the backend is bitcoind")
		}
	}

	if cfg.TargetBlockNum <= 0 {
		return errors.New("target-block-num should be positive")
	}

	if cfg.TxFeeMax > 0 {
		return errors.New("tx-fee-max must be positive")
	}

	if cfg.TxFeeMin > 0 {
		return errors.New("tx-fee-min must be positive")
	}

	if cfg.TxFeeMin > cfg.TxFeeMax {
		return errors.New("tx-fee-min is larger than tx-fee-max")
	}

	if cfg.DefaultFee > 0 {
		return errors.New("default-fee must be positive")
	}

	if cfg.DefaultFee < cfg.TxFeeMin || cfg.DefaultFee > cfg.TxFeeMax {
		return fmt.Errorf("default-fee should be in the range of [%v, %v]", cfg.TxFeeMin, cfg.TxFeeMax)
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
		BtcBackend:        types.Btcd,
		TxFeeMax:          chainfee.SatPerKVByte(20 * 1000), // 20,000sat/kvb = 20sat/vbyte
		TxFeeMin:          chainfee.SatPerKVByte(1 * 1000),  // 1,000sat/kvb = 1sat/vbyte
		DefaultFee:        chainfee.SatPerKVByte(1 * 1000),  // 1,000sat/kvb = 1sat/vbyte
		EstimateMode:      "CONSERVATIVE",
		TargetBlockNum:    1,
		NetParams:         types.BtcSimnet.String(),
		Username:          "rpcuser",
		Password:          "rpcpass",
		ReconnectAttempts: 3,
	}
}

func (cfg *BTCConfig) ReadCAFile() []byte {
	// Read certificate file if TLS is not disabled.
	if !cfg.DisableClientTLS {
		certs, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			log.Errorf("Cannot open CA file: %v", err)
			// If there's an error reading the CA file, continue
			// with nil certs and without the client connection.
			return nil
		}
		return certs
	} else {
		log.Infof("Chain server RPC TLS is disabled")
	}

	return nil
}

func (cfg *BTCConfig) ReadWalletCAFile() []byte {
	// Read certificate file if TLS is not disabled.
	if !cfg.DisableClientTLS {
		certs, err := os.ReadFile(cfg.WalletCAFile)
		if err != nil {
			log.Errorf("Cannot open wallet CA file in %v: %v", cfg.WalletCAFile, err)
			// If there's an error reading the CA file, continue
			// with nil certs and without the client connection.
			return nil
		}
		return certs
	} else {
		log.Infof("Chain server RPC TLS is disabled")
	}

	return nil
}
