package netparams

import (
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/types"
)

// TODO: add Babylon net params here
type BabylonParams struct {
	Tag     btctxformatter.BabylonTag
	Version btctxformatter.FormatVersion
}

func GetBabylonParams(net string, tagIdx uint8) *BabylonParams {
	switch net {
	case types.BtcMainnet.String():
		return &BabylonParams{
			Tag:     btctxformatter.MainTag(tagIdx),
			Version: btctxformatter.CurrentVersion,
		}
	case types.BtcTestnet.String():
		return &BabylonParams{
			Tag:     btctxformatter.TestTag(tagIdx),
			Version: btctxformatter.CurrentVersion,
		}
	case types.BtcSimnet.String():
		return &BabylonParams{
			Tag:     btctxformatter.TestTag(tagIdx),
			Version: btctxformatter.CurrentVersion,
		}
	case types.BtcRegtest.String():
		// bitcoind uses regtest instead of simnet
		return &BabylonParams{
			Tag:     btctxformatter.TestTag(tagIdx),
			Version: btctxformatter.CurrentVersion,
		}
	}

	return nil
}
