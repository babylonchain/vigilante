package reporter

import (
	"fmt"
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

	/* ensure BTC has catched up with BBN header chain */

	// Retrieve hash/height of the latest block in BTC
	btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		panic(err)
	}
	log.Infof("BTC latest block hash and height: (%v, %d)", btcLatestBlockHash, btcLatestBlockHeight)

	// TODO: if BTC falls behind BTCLightclient's base header, then the vigilante is incorrectly configured and should panic

	// Retrieve hash/height of the latest block in BBN header chain
	bbnLatestBlockHash, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		panic(err)
	}
	log.Infof("BBN header chain latest block hash and height: (%v, %d)", bbnLatestBlockHash, bbnLatestBlockHeight)

	// If BTC chain is shorter than BBN header chain, pause until BTC catches up
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

	/* Initialize BTC Cache */

	// Download all blocks since height T-k-w from BTC, where
	// - T is total block count in BBN header chain
	// - k is btcConfirmationDepth of BBN
	// - w is checkpointFinalizationTimeout of BBN
	if err = r.initBTCCache(); err != nil {
		panic(err)
	}

	/* Initial consistency check */

	// Find k-deep block of BBN header chain in BTC cache
	kDeepBBNBlockHeight := bbnLatestBlockHeight - r.btcConfirmationDepth + 1
	kDeepBBNBlock := r.btcCache.FindBlock(kDeepBBNBlockHeight)
	if kDeepBBNBlock == nil {
		log.Warnf("cannot find k-deep block (height: %d) in BBN header chain in BTC cache, skip initial consistency check", kDeepBBNBlockHeight)
	} else {
		kDeepBBNBlockHash := kDeepBBNBlock.BlockHash()
		consistent, err := r.babylonClient.QueryContainsBlock(&kDeepBBNBlockHash)
		if err != nil {
			panic(err)
		}
		if !consistent {
			err = fmt.Errorf("BTC main chain is inconsistent with BBN header chain: k-deep block in BBN header chain: %v", kDeepBBNBlockHash)
			// TODO: produce and forward inconsistency evidence to BBN, make BBN panic
			panic(err)
		}
	}

	/* help BBN to catch up with BTC */

	// Extract headers from BTC cache and forward them to BBN
	ibs := r.btcCache.GetLastBlocks(bbnLatestBlockHeight)
	signer := r.babylonClient.MustGetAddr()
	for _, ib := range ibs {
		blockHash := ib.BlockHash()
		if err = r.submitHeader(signer, ib.Header); err != nil {
			log.Errorf("Failed to handle header %v from Bitcoin: %v", blockHash, err)
		}

		// extract checkpoints into the pool
		r.extractCkpts(ib)
	}

	// Find matched checkpoint segments and submit checkpoints
	if err = r.matchAndSubmitCkpts(signer); err != nil {
		log.Errorf("Failed to match and submit checkpoints to BBN: %v", err)
	}
}

// initBTCCache fetches the last blocks in the BTC canonical chain
// TODO: make the BTC cache size a system parameter
func (r *Reporter) initBTCCache() error {
	var (
		err             error
		totalBlockCount uint64
		ibs             []*types.IndexedBlock
		btcCache        = r.btcCache
	)

	// get T, i.e., total block count in BBN header chain
	// TODO: now T is the height of BTC chain rather than BBN header chain
	_, totalBlockCount, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		return err
	}

	// Fetch block since `stopHeight = T - k - w` from BTC, where
	// - T is total block count in BBN header chain
	// - k is btcConfirmationDepth of BBN
	// - w is checkpointFinalizationTimeout of BBN
	stopHeight := int32(totalBlockCount) - int32(r.btcConfirmationDepth) - int32(r.checkpointFinalizationTimeout)
	if stopHeight < 0 { // this happens when Bitcoin contains less than `k+w` blocks
		stopHeight = 0
	}

	ibs, err = r.btcClient.GetLastBlocks(stopHeight)
	if err != nil {
		return err
	}

	return btcCache.Init(ibs)
}
