// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package types_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/babylon/testutil/datagen"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func newTestTx(babylonTx bool) *wire.MsgTx {
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				SignatureScript: []byte{
					0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62,
				},
				Sequence: 0xffffffff,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value: 0x12a05f200,
				PkScript: []byte{
					0x41, // OP_DATA_65
					0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
					0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
					0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
					0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
					0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
					0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
					0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
					0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
					0xa6, // 65-byte signature
					0xac, // OP_CHECKSIG
				},
			},
			{
				Value: 0x5f5e100,
				PkScript: []byte{
					0x41, // OP_DATA_65
					0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
					0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
					0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
					0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
					0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
					0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
					0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
					0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
					0xa6, // 65-byte signature
					0xac, // OP_CHECKSIG
				},
			},
		},
		LockTime: 0,
	}
	if babylonTx {
		// fake a raw checkpoint
		rawBTCCkpt := getRandomRawCheckpoint()
		// encode raw checkpoint to two halves
		firstHalf, secondHalf, err := btctxformatter.EncodeCheckpointData(
			btctxformatter.MainTag(48),
			btctxformatter.CurrentVersion,
			rawBTCCkpt,
		)
		if err != nil {
			panic(err)
		}
		idx := rand.Intn(2)
		if idx == 0 {
			tx.TxOut[0].PkScript = firstHalf
		} else {
			tx.TxOut[0].PkScript = secondHalf
		}

	}
	return tx
}

// calcMerkleRoot creates a merkle tree from the slice of transactions and
// returns the root of the tree.
func calcMerkleRoot(txns []*wire.MsgTx) chainhash.Hash {
	if len(txns) == 0 {
		return chainhash.Hash{}
	}

	utilTxns := make([]*btcutil.Tx, 0, len(txns))
	for _, tx := range txns {
		utilTxns = append(utilTxns, btcutil.NewTx(tx))
	}
	merkles := blockchain.BuildMerkleTreeStore(utilTxns, false)
	return *merkles[len(merkles)-1]
}

func genRandomBlocksWithBabylonTx(n int, percentage float32) ([]*types.IndexedBlock, []bool) {
	blocks := []*types.IndexedBlock{}
	isBabylonBlockArray := []bool{}
	// percentage should be [0, 1]
	if percentage < 0 || percentage > 1 {
		return blocks, isBabylonBlockArray
	}

	for i := 0; i < n; i++ {
		// create a tx, which will be a Babylon tx with probability `percentage`
		coinbaseTx := newTestTx(false)
		var msgTx *wire.MsgTx
		if rand.Float32() < percentage {
			msgTx = newTestTx(true)
			isBabylonBlockArray = append(isBabylonBlockArray, true)
		} else {
			msgTx = newTestTx(false)
			isBabylonBlockArray = append(isBabylonBlockArray, false)
		}
		msgTxs := []*wire.MsgTx{coinbaseTx, msgTx}
		merkleRoot := calcMerkleRoot(msgTxs)

		// construct and append block
		header := datagen.GenRandomBtcdHeader()
		header.MerkleRoot = merkleRoot
		ib := types.NewIndexedBlock(rand.Int31(), header, []*btcutil.Tx{btcutil.NewTx(coinbaseTx), btcutil.NewTx(msgTx)})
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

			parsedProof, err := btcctypes.ParseProof(
				spvProof.BtcTransaction,
				spvProof.BtcTransactionIndex,
				spvProof.MerkleNodes,
				spvProof.ConfirmingBtcHeader,
				chaincfg.SimNetParams.PowLimit,
			)

			if isBabylonBlockArray[i] {
				require.NotNil(t, parsedProof)
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
