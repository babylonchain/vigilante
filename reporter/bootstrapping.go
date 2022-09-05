package reporter

import (
	"time"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func (r *Reporter) Init() {
	var (
		btcLatestBlockHash   *chainhash.Hash
		btcLatestBlockHeight int32
		bbnLatestBlockHash   *chainhash.Hash
		bbnLatestBlockHeight uint64
		err                  error
	)

	// TODO: retrieve k and w

	// retrieve hash/height of the latest block in BTC
	btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		panic(err)
	}
	log.Infof("BTC latest block hash and height: (%v, %d)", btcLatestBlockHash, btcLatestBlockHeight)

	// retrieve hash/height of the latest block in BBN header chain
	bbnLatestBlockHash, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		panic(err)
	}
	log.Infof("BBN header chain latest block hash and height: (%v, %d)", bbnLatestBlockHash, bbnLatestBlockHeight)

	// if BTC chain is shorter than BBN header chain, pause until BTC catches up
	if uint64(btcLatestBlockHeight) < bbnLatestBlockHeight {
		log.Infof("BTC chain (length %d) falls behind BBN header chain (length %d), wait until BTC catches up", btcLatestBlockHeight, bbnLatestBlockHeight)

		// periodically check if BTC catches up with BBN.
		// When BTC catches up, break and continue the bootstrapping process
		ticker := time.NewTicker(10 * time.Second) // TODO: parameterise the polling interval
		for range ticker.C {
			btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
			if err != nil {
				panic(err)
			}
			bbnLatestBlockHash, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
			if err != nil {
				panic(err)
			}
			if uint64(btcLatestBlockHeight) >= bbnLatestBlockHeight {
				log.Infof("BTC chain (length %d) now catches up with BBN header chain (length %d), continue bootstrapping", btcLatestBlockHeight, bbnLatestBlockHeight)
				break
			}
		}
	}

	// TODO: initial consistency check

	// Initialize BTC Cache
	if err := r.initBTCCache(); err != nil {
		panic(err)
	}

	// TODO: extract headers from BTC cache and forward them to BBN

	// TODO: extract ckpt segments from BTC cache, store them in ckpt segment pool, check newly matched ckpts, and forward newly matched ckpts to BBN
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
