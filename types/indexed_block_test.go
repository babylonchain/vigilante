// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package types_test

import (
	"compress/bzip2"
	"encoding/binary"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/stretchr/testify/require"
)

// loadBlocks reads files containing bitcoin block data (gzipped but otherwise
// in the format bitcoind writes) from disk and returns them as an array of
// btcutil.Block.  This is largely borrowed from the test code in btcdb.
// copied from https://github.com/btcsuite/btcd/blob/master/blockchain/common_test.go
func loadBlocks(filename string) (blocks []*btcutil.Block, err error) {
	filename = filepath.Join("btctestdata/", filename)

	var network = wire.MainNet
	var dr io.Reader
	var fi io.ReadCloser

	fi, err = os.Open(filename)
	if err != nil {
		return
	}

	if strings.HasSuffix(filename, ".bz2") {
		dr = bzip2.NewReader(fi)
	} else {
		dr = fi
	}
	defer fi.Close()

	var block *btcutil.Block

	err = nil
	for height := int64(1); err == nil; height++ {
		var rintbuf uint32
		err = binary.Read(dr, binary.LittleEndian, &rintbuf)
		if err == io.EOF {
			// hit end of file at expected offset: no warning
			height--
			err = nil
			break
		}
		if err != nil {
			break
		}
		if rintbuf != uint32(network) {
			break
		}
		err = binary.Read(dr, binary.LittleEndian, &rintbuf)
		blocklen := rintbuf

		rbytes := make([]byte, blocklen)

		// read block
		dr.Read(rbytes)

		block, err = btcutil.NewBlockFromBytes(rbytes)
		if err != nil {
			return
		}
		blocks = append(blocks, block)
	}

	return
}

func FuzzIndexedBlock(f *testing.F) {
	testFiles := []string{
		"blk_0_to_4.dat.bz2",
		"blk_3A.dat.bz2",
	}

	// get a list of IndexedBlock from testdata
	var blocks []*types.IndexedBlock
	for _, file := range testFiles {
		tmpBlocks, err := loadBlocks(file)
		if err != nil {
			f.Errorf("Error loading file: %v\n", err)
			return
		}
		for _, tmpBlock := range tmpBlocks {
			height := tmpBlock.Height()
			header := &tmpBlock.MsgBlock().Header
			txs := types.GetWrappedTxs(tmpBlock.MsgBlock())
			indexedBlock := types.NewIndexedBlock(height, header, txs)
			blocks = append(blocks, indexedBlock)
		}
	}

	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		for _, block := range blocks {
			// skip blocks with no tx
			if len(block.Txs) == 0 {
				continue
			}
			idx := rand.Int() % len(block.Txs)

			spvProof, err := block.GenSPVProof(idx)
			require.NoError(t, err)

			parsedProof, err := btcctypes.ParseProof(
				spvProof.BtcTransaction,
				spvProof.BtcTransactionIndex,
				spvProof.MerkleNodes,
				spvProof.ConfirmingBtcHeader,
				chaincfg.SimNetParams.PowLimit,
			)

			// ParseProof requires the tx to have OP_RETURN data, which is out of the scope of verifying Merkle proofs
			if strings.Contains(err.Error(), "provided transaction should provide op return data") {
				continue
			}
			require.NotNil(t, parsedProof)
			require.NoError(t, err)
		}
	})
}
