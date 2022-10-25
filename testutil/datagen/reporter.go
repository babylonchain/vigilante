package datagen

import (
	"math/big"
	"math/rand"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/babylon/testutil/datagen"
	babylontypes "github.com/babylonchain/babylon/types"
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

func GenRandomBabylonTx() *wire.MsgTx {
	tx := GenRandomTx()
	builder := txscript.NewScriptBuilder()

	// fake a raw checkpoint
	rawBTCCkpt := GetRandomRawBtcCheckpoint()
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

func GenRandomBlock(babylonBlock bool) *wire.MsgBlock {
	// create a tx, which will be a Babylon tx with probability `percentage`
	var msgTx *wire.MsgTx
	if babylonBlock {
		msgTx = GenRandomBabylonTx()
	} else {
		msgTx = GenRandomTx()
	}
	coinbaseTx := GenRandomTx()
	msgTxs := []*wire.MsgTx{coinbaseTx, msgTx}

	var header *wire.BlockHeader

	// calculate correct Merkle root
	merkleRoot := calcMerkleRoot(msgTxs)
	// don't apply any difficulty
	difficulty, _ := new(big.Int).SetString("7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)
	workBits := blockchain.BigToCompact(difficulty)

	for {
		header = datagen.GenRandomBtcdHeader()
		header.MerkleRoot = merkleRoot
		header.Bits = workBits

		if err := babylontypes.ValidateBTCHeader(header, chaincfg.SimNetParams.PowLimit); err == nil {
			break
		}
	}

	block := &wire.MsgBlock{
		Header:       *header,
		Transactions: []*wire.MsgTx{coinbaseTx, msgTx},
	}
	return block
}
