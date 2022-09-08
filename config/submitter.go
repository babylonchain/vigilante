package config

import (
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/btcutil"
)

// SubmitterConfig defines configuration for the gRPC-web server.
type SubmitterConfig struct {
	NetParams  string         `mapstructure:"netparams"` // should be mainnet|testnet|simnet
	TxFee      btcutil.Amount `mapstructure:txfee`
	BufferSize int
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
	amount, err := btcutil.NewAmount(0.00001)
	if err != nil {
		panic(err)
	}
	return SubmitterConfig{
		NetParams:  "simnet",
		TxFee:      amount,
		BufferSize: 100,
	}
}
