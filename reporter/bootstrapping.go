package reporter

import (
	"time"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

func (r *Reporter) Init() {
	var (
		btcLatestBlockHash   *chainhash.Hash
		btcLatestBlockHeight int32
		bbnLatestBlockHash   *chainhash.Hash
		bbnLatestBlockHeight uint64
		err                  error
	)

	// Download h-w blocks and initialize BTC Cache
	if err = r.initBTCCache(); err != nil {
		panic(err)
	}

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

	// get the last BTC blocks since height h-w, where h is the height of the k-deep block in BBN main chain
	kDeepBBNBlockHeight := bbnLatestBlockHeight - r.btcConfirmationDepth + 1 // h
	ibs := r.btcCache.GetLastBlocks(kDeepBBNBlockHeight - r.checkpointFinalizationTimeout)

	// initial consistency check
	// get k-deep height in BBN as h, check if BTCChain[h] exists in BBN header chain
	kDeepBBNBlockHash := ibs[r.checkpointFinalizationTimeout-1].BlockHash()
	consistent, err := r.babylonClient.QueryContainsBlock(&kDeepBBNBlockHash)
	if err != nil {
		panic(err)
	}
	if !consistent {
		log.Errorf("BTC main chain is inconsistent with BBN header chain")
		log.Debugf("k-deep block in BBN header chain: %v", kDeepBBNBlockHash)
		// TODO: produce and forward inconsistency evidence to BBN, make BBN panic
	}

	// Extract headers from BTC cache and forward them to BBN
	signer := r.babylonClient.MustGetAddr()
	for _, ib := range ibs {
		blockHash := ib.BlockHash()
		if err = r.submitHeader(signer, ib.Header); err != nil {
			log.Errorf("Failed to handle header %v from Bitcoin: %v", blockHash, err)
		}
		// extract ckpts into the pool
		r.extractCkpts(ib)
	}

	// find matched ckpt segments and submit ckpts
	if err := r.matchAndSubmitCkpts(signer); err != nil {
		log.Errorf("Failed to match and submit checkpoints to BBN: %v", err)
	}
}

// initBTCCache fetches the last blocks in the BTC canonical chain
// TODO: make the BTC cache size a system parameter
func (r *Reporter) initBTCCache() error {
	var (
		err             error
		totalBlockCount int64
		ibs             []*types.IndexedBlock
		btcCache        = r.btcCache
	)

	totalBlockCount, err = r.btcClient.GetBlockCount()
	if err != nil {
		return err
	}

	// Fetch h - w blocks where h is height of K deep block
	kDeepBlockHeight := uint64(totalBlockCount) - r.btcConfirmationDepth
	stopHeight := int32(kDeepBlockHeight - r.checkpointFinalizationTimeout)

	ibs, err = r.btcClient.GetLastBlocks(stopHeight)
	if err != nil {
		return err
	}

	return btcCache.Init(ibs)
}
