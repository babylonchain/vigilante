package datagen

import (
	"math/big"
	"math/rand"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/babylon/testutil/datagen"
	babylontypes "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// calcMerkleRoot creates a merkle tree from the slice of transactions and
// returns the root of the tree.
// (taken from https://github.com/btcsuite/btcd/blob/master/blockchain/fullblocktests/generate.go)
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

func GetRandomRawBtcCheckpoint() *btctxformatter.RawBtcCheckpoint {
	return &btctxformatter.RawBtcCheckpoint{
		Epoch:            rand.Uint64(),
		LastCommitHash:   datagen.GenRandomByteArray(btctxformatter.LastCommitHashLength),
		BitMap:           datagen.GenRandomByteArray(btctxformatter.BitMapLength),
		SubmitterAddress: datagen.GenRandomByteArray(btctxformatter.AddressLength),
		BlsSig:           datagen.GenRandomByteArray(btctxformatter.BlsSigLength),
	}
}

func GenRandomTx() *wire.MsgTx {
	// structure of the below tx is from https://github.com/btcsuite/btcd/blob/master/wire/msgtx_test.go
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.HashH(datagen.GenRandomByteArray(10)),
					Index: rand.Uint32(),
				},
				SignatureScript: datagen.GenRandomByteArray(10),
				Sequence:        rand.Uint32(),
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    rand.Int63(),
				PkScript: datagen.GenRandomByteArray(80),
			},
		},
		LockTime: 0,
	}

	return tx
}

func GenRandomBabylonTxPair() ([]*wire.MsgTx, *btctxformatter.RawBtcCheckpoint) {
	txs := []*wire.MsgTx{GenRandomTx(), GenRandomTx()}
	builder := txscript.NewScriptBuilder()

	// fake a raw checkpoint
	rawBTCCkpt := GetRandomRawBtcCheckpoint()
	// encode raw checkpoint to two halves
	firstHalf, secondHalf, err := btctxformatter.EncodeCheckpointData(
		btctxformatter.TestTag(48),
		btctxformatter.CurrentVersion,
		rawBTCCkpt,
	)
	if err != nil {
		panic(err)
	}

	dataScript1, err := builder.AddOp(txscript.OP_RETURN).AddData(firstHalf).Script()
	if err != nil {
		panic(err)
	}
	txs[0].TxOut[0] = wire.NewTxOut(0, dataScript1)

	// reset builder
	builder = txscript.NewScriptBuilder()

	dataScript2, err := builder.AddOp(txscript.OP_RETURN).AddData(secondHalf).Script()
	if err != nil {
		panic(err)
	}
	txs[1].TxOut[0] = wire.NewTxOut(0, dataScript2)

	return txs, rawBTCCkpt
}

func GenRandomBabylonTx() *wire.MsgTx {
	tx := GenRandomTx()
	builder := txscript.NewScriptBuilder()

	// fake a raw checkpoint
	rawBTCCkpt := GetRandomRawBtcCheckpoint()
	// encode raw checkpoint to two halves
	firstHalf, secondHalf, err := btctxformatter.EncodeCheckpointData(
		btctxformatter.TestTag(48),
		btctxformatter.CurrentVersion,
		rawBTCCkpt,
	)
	if err != nil {
		panic(err)
	}
	idx := rand.Intn(2)
	if idx == 0 {
		dataScript, err := builder.AddOp(txscript.OP_RETURN).AddData(firstHalf).Script()
		if err != nil {
			panic(err)
		}
		tx.TxOut[0] = wire.NewTxOut(0, dataScript)
	} else {
		dataScript, err := builder.AddOp(txscript.OP_RETURN).AddData(secondHalf).Script()
		if err != nil {
			panic(err)
		}
		tx.TxOut[0] = wire.NewTxOut(0, dataScript)
	}

	return tx
}

func GenRandomBlock(numBabylonTxs int, prevHash *chainhash.Hash) (*wire.MsgBlock, *btctxformatter.RawBtcCheckpoint) {
	// create a tx, which will be a Babylon tx with probability `percentage`
	var (
		randomTxs []*wire.MsgTx
		rawCkpt   *btctxformatter.RawBtcCheckpoint
	)

	if numBabylonTxs == 2 {
		randomTxs, rawCkpt = GenRandomBabylonTxPair()
	} else if numBabylonTxs == 1 {
		randomTxs, _ = GenRandomBabylonTxPair()
		randomTxs[1] = GenRandomTx()
		rawCkpt = nil
	} else {
		randomTxs = []*wire.MsgTx{GenRandomTx(), GenRandomTx()}
		rawCkpt = nil
	}
	coinbaseTx := GenRandomTx()
	msgTxs := []*wire.MsgTx{coinbaseTx}
	msgTxs = append(msgTxs, randomTxs...)

	// calculate correct Merkle root
	merkleRoot := calcMerkleRoot(msgTxs)
	// don't apply any difficulty
	difficulty, _ := new(big.Int).SetString("7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)
	workBits := blockchain.BigToCompact(difficulty)

	// find a header that satisfies difficulty
	var header *wire.BlockHeader
	for {
		header = datagen.GenRandomBtcdHeader()
		header.MerkleRoot = merkleRoot
		header.Bits = workBits

		if prevHash == nil {
			header.PrevBlock = chainhash.DoubleHashH(datagen.GenRandomByteArray(10))
		} else {
			header.PrevBlock = *prevHash
		}

		if err := babylontypes.ValidateBTCHeader(header, chaincfg.SimNetParams.PowLimit); err == nil {
			break
		}
	}

	block := &wire.MsgBlock{
		Header:       *header,
		Transactions: msgTxs,
	}
	return block, rawCkpt
}

func GetRandomIndexedBlocks(numBlocks uint64) []*types.IndexedBlock {
	blocks, _, _ := GenRandomBlockchainWithBabylonTx(numBlocks, 0, 0)
	var ibs []*types.IndexedBlock

	blockHeight := rand.Int31() + int32(numBlocks)
	for _, block := range blocks {
		ibs = append(ibs, types.NewIndexedBlockFromMsgBlock(blockHeight, block))
		blockHeight -= 1
	}

	return ibs
}

func GenRandomBlockchainWithBabylonTx(n uint64, partialPercentage float32, fullPercentage float32) ([]*wire.MsgBlock, int, []*btctxformatter.RawBtcCheckpoint) {
	blocks := []*wire.MsgBlock{}
	numCkptSegs := 0
	rawCkpts := []*btctxformatter.RawBtcCheckpoint{}
	// percentage should be [0, 1]
	if partialPercentage < 0 || partialPercentage > 1 {
		return blocks, 0, rawCkpts
	}
	if fullPercentage < 0 || fullPercentage > 1 {
		return blocks, 0, rawCkpts
	}
	// n should be > 0
	if n == 0 {
		return blocks, 0, rawCkpts
	}

	// genesis block
	genesisBlock, rawCkpt := GenRandomBlock(0, nil)
	blocks = append(blocks, genesisBlock)
	rawCkpts = append(rawCkpts, rawCkpt)

	// blocks after genesis
	for i := uint64(1); i < n; i++ {
		var msgBlock *wire.MsgBlock
		prevHash := blocks[len(blocks)-1].BlockHash()
		if rand.Float32() < partialPercentage {
			msgBlock, rawCkpt = GenRandomBlock(1, &prevHash)
			numCkptSegs += 1
		} else if rand.Float32() < partialPercentage+fullPercentage {
			msgBlock, rawCkpt = GenRandomBlock(2, &prevHash)
			numCkptSegs += 2
		} else {
			msgBlock, rawCkpt = GenRandomBlock(0, &prevHash)
		}

		blocks = append(blocks, msgBlock)
		rawCkpts = append(rawCkpts, rawCkpt)
	}
	return blocks, numCkptSegs, rawCkpts
}
