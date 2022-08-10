package netparams

import (
	"github.com/btcsuite/btcd/chaincfg"
)

func GetBTCParams(net string) *chaincfg.Params {
	switch net {
	case "mainnet":
		return &chaincfg.MainNetParams
	case "testnet":
		return &chaincfg.TestNet3Params
	case "signet":
		return &chaincfg.SigNetParams
	case "simnet":
		return &chaincfg.SimNetParams
	default:
		return &chaincfg.SimNetParams
	}
}
