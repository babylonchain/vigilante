package datagen

import (
	"math"
	"math/big"
	"math/rand"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/babylon/testutil/datagen"
	babylontypes "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
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

func GetRandomRawBtcCheckpoint(r *rand.Rand) *btctxformatter.RawBtcCheckpoint {
	return &btctxformatter.RawBtcCheckpoint{
		Epoch:            r.Uint64(),
		LastCommitHash:   datagen.GenRandomByteArray(r, btctxformatter.LastCommitHashLength),
		BitMap:           datagen.GenRandomByteArray(r, btctxformatter.BitMapLength),
		SubmitterAddress: datagen.GenRandomByteArray(r, btctxformatter.AddressLength),
		BlsSig:           datagen.GenRandomByteArray(r, btctxformatter.BlsSigLength),
	}
}

func GenRandomTx(r *rand.Rand) *wire.MsgTx {
	// structure of the below tx is from https://github.com/btcsuite/btcd/blob/master/wire/msgtx_test.go
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.HashH(datagen.GenRandomByteArray(r, 10)),
					Index: r.Uint32(),
				},
				SignatureScript: datagen.GenRandomByteArray(r, 10),
				Sequence:        r.Uint32(),
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    r.Int63(),
				PkScript: datagen.GenRandomByteArray(r, 80),
			},
		},
		LockTime: 0,
	}

	return tx
}

func GenRandomBabylonTxPair(r *rand.Rand) ([]*wire.MsgTx, *btctxformatter.RawBtcCheckpoint) {
	txs := []*wire.MsgTx{GenRandomTx(r), GenRandomTx(r)}
	builder := txscript.NewScriptBuilder()

	// fake a raw checkpoint
	rawBTCCkpt := GetRandomRawBtcCheckpoint(r)
	// encode raw checkpoint to two halves
	firstHalf, secondHalf, err := btctxformatter.EncodeCheckpointData(
		[]byte{1, 2, 3, 4},
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

func GenRandomBabylonTx(r *rand.Rand) *wire.MsgTx {
	tx := GenRandomTx(r)
	builder := txscript.NewScriptBuilder()

	// fake a raw checkpoint
	rawBTCCkpt := GetRandomRawBtcCheckpoint(r)
	// encode raw checkpoint to two halves
	firstHalf, secondHalf, err := btctxformatter.EncodeCheckpointData(
		[]byte{1, 2, 3, 4},
		btctxformatter.CurrentVersion,
		rawBTCCkpt,
	)
	if err != nil {
		panic(err)
	}
	idx := r.Intn(2)
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

func GenRandomBlock(r *rand.Rand, numBabylonTxs int, prevHash *chainhash.Hash) (*wire.MsgBlock, *btctxformatter.RawBtcCheckpoint) {
	// create a tx, which will be a Babylon tx with probability `percentage`
	var (
		randomTxs []*wire.MsgTx
		rawCkpt   *btctxformatter.RawBtcCheckpoint
	)

	if numBabylonTxs == 2 {
		randomTxs, rawCkpt = GenRandomBabylonTxPair(r)
	} else if numBabylonTxs == 1 {
		randomTxs, _ = GenRandomBabylonTxPair(r)
		randomTxs[1] = GenRandomTx(r)
		rawCkpt = nil
	} else {
		randomTxs = []*wire.MsgTx{GenRandomTx(r), GenRandomTx(r)}
		rawCkpt = nil
	}
	coinbaseTx := GenRandomTx(r)
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
		header = datagen.GenRandomBtcdHeader(r)
		header.MerkleRoot = merkleRoot
		header.Bits = workBits

		if prevHash == nil {
			header.PrevBlock = chainhash.DoubleHashH(datagen.GenRandomByteArray(r, 10))
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

// GetRandomIndexedBlocks generates a random number of indexed blocks with a random root height
func GetRandomIndexedBlocks(r *rand.Rand, numBlocks uint64) []*types.IndexedBlock {
	var ibs []*types.IndexedBlock

	if numBlocks == 0 {
		return ibs
	}

	block, _ := GenRandomBlock(r, 1, nil)
	prevHeight := r.Int31n(math.MaxInt32 - int32(numBlocks))
	ib := types.NewIndexedBlockFromMsgBlock(prevHeight, block)
	prevHash := ib.Header.BlockHash()

	ibs = GetRandomIndexedBlocksFromHeight(r, numBlocks-1, prevHeight, prevHash)
	ibs = append([]*types.IndexedBlock{ib}, ibs...)
	return ibs
}

// GetRandomIndexedBlocksFromHeight generates a random number of indexed blocks with a given root height and root hash
func GetRandomIndexedBlocksFromHeight(r *rand.Rand, numBlocks uint64, rootHeight int32, rootHash chainhash.Hash) []*types.IndexedBlock {
	var (
		ibs        []*types.IndexedBlock
		prevHash   = rootHash
		prevHeight = rootHeight
	)

	for i := 0; i < int(numBlocks); i++ {
		block, _ := GenRandomBlock(r, 1, &prevHash)
		newIb := types.NewIndexedBlockFromMsgBlock(prevHeight+1, block)
		ibs = append(ibs, newIb)

		prevHeight = newIb.Height
		prevHash = newIb.Header.BlockHash()
	}

	return ibs
}

func GenRandomBlockchainWithBabylonTx(r *rand.Rand, n uint64, partialPercentage float32, fullPercentage float32) ([]*wire.MsgBlock, int, []*btctxformatter.RawBtcCheckpoint) {
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
	genesisBlock, rawCkpt := GenRandomBlock(r, 0, nil)
	blocks = append(blocks, genesisBlock)
	rawCkpts = append(rawCkpts, rawCkpt)

	// blocks after genesis
	for i := uint64(1); i < n; i++ {
		var msgBlock *wire.MsgBlock
		prevHash := blocks[len(blocks)-1].BlockHash()
		if r.Float32() < partialPercentage {
			msgBlock, rawCkpt = GenRandomBlock(r, 1, &prevHash)
			numCkptSegs += 1
		} else if r.Float32() < partialPercentage+fullPercentage {
			msgBlock, rawCkpt = GenRandomBlock(r, 2, &prevHash)
			numCkptSegs += 2
		} else {
			msgBlock, rawCkpt = GenRandomBlock(r, 0, &prevHash)
		}

		blocks = append(blocks, msgBlock)
		rawCkpts = append(rawCkpts, rawCkpt)
	}
	return blocks, numCkptSegs, rawCkpts
}
