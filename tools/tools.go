//go:build tools
// +build tools

package vigilante

import (
	_ "github.com/babylonchain/babylon/cmd/babylond"
	_ "github.com/btcsuite/btcd"
	_ "github.com/btcsuite/btcwallet"
)
