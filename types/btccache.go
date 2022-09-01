package types

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

type BTCCache struct {
	blocks     []*IndexedBlock
	maxEntries uint
}

func NewBTCCache(maxEntries uint) *BTCCache {
	return &BTCCache{
		blocks:     make([]*IndexedBlock, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

func (b *BTCCache) Add(ib *IndexedBlock) {
	if b.maxEntries == 0 {
		return
	}

	if uint(len(b.blocks)) == b.maxEntries {
		b.blocks = b.blocks[1:]
	}

	b.blocks = append(b.blocks, ib)
}

func (b *BTCCache) reverse() {
	for i, j := 0, len(b.blocks)-1; i < j; i, j = i+1, j-1 {
		b.blocks[i], b.blocks[j] = b.blocks[j], b.blocks[i]
	}
}

func (b *BTCCache) Init(client *rpcclient.Client) error {
	var (
		err             error
		prevBlockHash   *chainhash.Hash
		blockInfo       *btcjson.GetBlockVerboseResult
		mBlock          *wire.MsgBlock
		totalBlockCount int64
		maxEntries      = b.maxEntries
	)

	prevBlockHash, _, err = client.GetBestBlock()
	if err != nil {
		return err
	}

	totalBlockCount, err = client.GetBlockCount()
	if err != nil {
		return err
	}

	if uint(totalBlockCount) < maxEntries {
		maxEntries = uint(totalBlockCount)
	}

	for uint(len(b.blocks)) < maxEntries {
		blockInfo, err = client.GetBlockVerbose(prevBlockHash)
		if err != nil {
			return err
		}

		mBlock, err = client.GetBlock(prevBlockHash)
		if err != nil {
			return err
		}

		btcTxs := getWrappedTxs(mBlock)
		ib := NewIndexedBlock(int32(blockInfo.Height), &mBlock.Header, btcTxs)

		b.blocks = append(b.blocks, ib)
		prevBlockHash = &mBlock.Header.PrevBlock
	}

	// Reverse cache in place to maintain ordering
	b.reverse()

	return nil
}
