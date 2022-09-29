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

func GetBabylonParams(net string, tagIdx uint8) *BabylonParams {
	switch net {
	case "mainnet":
		return &BabylonParams{
			Tag:     btctxformatter.MainTag(tagIdx),
			Version: btctxformatter.CurrentVersion,
		}
	case "testnet":
		return &BabylonParams{
			Tag:     btctxformatter.TestTag(tagIdx),
			Version: btctxformatter.CurrentVersion,
		}
	case "simnet":
		return &BabylonParams{
			Tag:     btctxformatter.TestTag(tagIdx),
			Version: btctxformatter.CurrentVersion,
		}
	default:
		panic(fmt.Errorf("babylon network should be one of {mainnet, testnet, simnet}"))
	}
}
