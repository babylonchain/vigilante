package reporter

import (
	"time"

	"github.com/babylonchain/vigilante/types"
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

	// retrieve k and w within btccParams
	btccParams, err := r.babylonClient.QueryBTCCheckpointParams()
	if err != nil {
		panic(err)
	}
	btcConfirmationDepth := btccParams.BtcConfirmationDepth                   // k
	checkpointFinalizationTimeout := btccParams.CheckpointFinalizationTimeout // w
	log.Infof("BTCCheckpoint parameters: (k, w) = (%d, %d)", btcConfirmationDepth, checkpointFinalizationTimeout)

	// retrieve hash/height of the latest block in BTC
	btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		panic(err)
	}
	log.Infof("BTC latest block hash and height: (%v, %d)", btcLatestBlockHash, btcLatestBlockHeight)

	// TODO: if BTC falls behind BTCLightclient's base header, then the vigilante is incorrectly configured and should panic

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
		ticker := time.NewTicker(5 * time.Second) // TODO: parameterise the polling interval
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
			log.Infof("BTC chain (length %d) still falls behind BBN header chain (length %d), keep waiting", btcLatestBlockHeight, bbnLatestBlockHeight)
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
		mBlock          *wire.MsgBlock
		blockHeight     int32
		totalBlockCount int32
		ibs             []*types.IndexedBlock
		btcCache        = r.btcCache
		maxEntries      = r.Cfg.BTCCacheMaxEntries
	)

	prevBlockHash, blockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		return err
	}

	// in case if the number of blocks is less than `maxEntries`
	totalBlockCount = blockHeight + 1
	if uint(totalBlockCount) < maxEntries {
		maxEntries = uint(totalBlockCount)
	}

	// retrieve the latest `maxEntries` blocks from BTC
	for i := uint(0); i < maxEntries; i++ {
		ib, err := r.btcClient.GetBlockByHash(prevBlockHash)
		if err != nil {
			return err
		}
		ibs = append(ibs, ib)
		prevBlockHash = &mBlock.Header.PrevBlock
	}

	btcCache.Init(ibs)

	return nil
}
