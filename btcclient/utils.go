package btcclient

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// IndexedBlock is a BTC block with some extra information compared to wire.MsgBlock, including:
// - block height
// - txHash, txHashWitness, txIndex for each Tx
// These are necessary for generating Merkle proof (and thus the `MsgInsertBTCSpvProof` message in babylon) of a certain tx
type IndexedBlock struct {
	Height int32
	Header *wire.BlockHeader
	Txs    []*btcutil.Tx
}

func NewIndexedBlock(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) *IndexedBlock {
	return &IndexedBlock{height, header, txs}
}

func (ib *IndexedBlock) MsgBlock() *wire.MsgBlock {
	msgTxs := []*wire.MsgTx{}
	for _, tx := range ib.Txs {
		msgTxs = append(msgTxs, tx.MsgTx())
	}

	return &wire.MsgBlock{
		Header:       *ib.Header,
		Transactions: msgTxs,
	}
}

func (ib *IndexedBlock) BlockHash() chainhash.Hash {
	return ib.Header.BlockHash()
}

// TODO: function for generating Merkle proof of a certain tx
