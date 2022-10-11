package netparams

import (
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg"
)

func GetBTCParams(net string) *chaincfg.Params {
	switch net {
	case types.BtcMainnet.String():
		return &chaincfg.MainNetParams
	case types.BtcTestnet.String():
		return &chaincfg.TestNet3Params
	case types.BtcSimnet.String():
		return &chaincfg.SimNetParams
	case types.BtcSignet.String():
		return &chaincfg.SigNetParams
	}
	return nil
}
