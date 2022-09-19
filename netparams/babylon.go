package netparams

import (
	"fmt"

	"github.com/babylonchain/babylon/btctxformatter"
)

// TODO: add Babylon net params here
type BabylonParams struct {
	Tag     btctxformatter.BabylonTag
	Version btctxformatter.FormatVersion
}

var BabylonMainNetParams = BabylonParams{
	Tag:     btctxformatter.MainTag(),
	Version: btctxformatter.CurrentVersion,
}

var BabylonTestNetParams = BabylonParams{
	Tag:     btctxformatter.TestTag(48),
	Version: btctxformatter.CurrentVersion,
}

var BabylonSimNetParams = BabylonParams{
	Tag:     btctxformatter.TestTag(48),
	Version: btctxformatter.CurrentVersion,
}

func GetBabylonParams(net string) *BabylonParams {
	switch net {
	case "mainnet":
		return &BabylonMainNetParams
	case "testnet":
		return &BabylonTestNetParams
	case "simnet":
		return &BabylonSimNetParams
	default:
		panic(fmt.Errorf("babylon network should be one of {mainnet, testnet, simnet}"))
	}
}
