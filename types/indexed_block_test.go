// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package types_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	vdatagen "github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func genRandomBlocksWithBabylonTx(n int, percentage float32) ([]*types.IndexedBlock, []bool) {
	blocks := []*types.IndexedBlock{}
	isBabylonBlockArray := []bool{}
	// percentage should be [0, 1]
	if percentage < 0 || percentage > 1 {
		return blocks, isBabylonBlockArray
	}

	for i := 0; i < n; i++ {
		var msgBlock *wire.MsgBlock
		if rand.Float32() < percentage {
			msgBlock = vdatagen.GenRandomBlockWithBabylonTx(true)
			isBabylonBlockArray = append(isBabylonBlockArray, true)
		} else {
			msgBlock = vdatagen.GenRandomBlockWithBabylonTx(false)
			isBabylonBlockArray = append(isBabylonBlockArray, false)
		}

		ib := types.NewIndexedBlock(rand.Int31(), &msgBlock.Header, types.GetWrappedTxs(msgBlock))
		blocks = append(blocks, ib)
	}
	return blocks, isBabylonBlockArray
}

func FuzzIndexedBlock(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		blocks, isBabylonBlockArray := genRandomBlocksWithBabylonTx(100, 0.4)
		for i, block := range blocks {
			// skip blocks with no tx
			if len(block.Txs) == 0 {
				continue
			}
			idx := rand.Intn(len(block.Txs))

			spvProof, err := block.GenSPVProof(idx)
			require.NoError(t, err)
			_, err = btcctypes.ParseProof(
				spvProof.BtcTransaction,
				spvProof.BtcTransactionIndex,
				spvProof.MerkleNodes,
				spvProof.ConfirmingBtcHeader,
				chaincfg.SimNetParams.PowLimit,
			)

			if isBabylonBlockArray[i] {
				// require.NotNil(t, parsedProof)
				require.NoError(t, err)
			} else {
				// ParseProof requires the tx to have OP_RETURN data, which is out of the scope of verifying Merkle proofs
				if strings.Contains(err.Error(), "provided transaction should provide op return data") {
					continue
				}
			}
		}
	})
}
