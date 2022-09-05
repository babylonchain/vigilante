package reporter

import (
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func (r *Reporter) Init() {
	// Initialize BTC Cache
	if err := r.initBTCCache(); err != nil {
		panic(err)
	}
}

// initBTCCache fetches the last blocks in the BTC canonical chain
// TODO: make the BTC cache size a system parameter
func (r *Reporter) initBTCCache() error {
	var (
		err             error
		prevBlockHash   *chainhash.Hash
		blockInfo       *btcjson.GetBlockVerboseResult
		mBlock          *wire.MsgBlock
		blockHeight     int32
		totalBlockCount int32
		btcCache        = r.btcCache
		maxEntries      = r.btcCache.MaxEntries
	)

	prevBlockHash, blockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		return err
	}

	totalBlockCount = blockHeight + 1

	if uint(totalBlockCount) < maxEntries {
		maxEntries = uint(totalBlockCount)
	}

	for uint(btcCache.Size()) < maxEntries {
		blockInfo, err = r.btcClient.GetBlockVerbose(prevBlockHash)
		if err != nil {
			return err
		}

		mBlock, err = r.btcClient.GetBlock(prevBlockHash)
		if err != nil {
			return err
		}

		btcTxs := types.GetWrappedTxs(mBlock)
		ib := types.NewIndexedBlock(int32(blockInfo.Height), &mBlock.Header, btcTxs)

		btcCache.Add(ib)
		prevBlockHash = &mBlock.Header.PrevBlock
	}

	// Reverse cache in place to maintain ordering
	btcCache.Reverse()

	return nil
}
