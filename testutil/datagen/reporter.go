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

func GenRandomTx(babylonTx bool) *wire.MsgTx {
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
		},
		LockTime: 0,
	}

	builder := txscript.NewScriptBuilder()

	if babylonTx {
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

	}
	return tx
}

func GenRandomBlock(babylonBlock bool) *wire.MsgBlock {
	// create a tx, which will be a Babylon tx with probability `percentage`
	coinbaseTx := GenRandomTx(false)
	msgTx := GenRandomTx(babylonBlock)
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
