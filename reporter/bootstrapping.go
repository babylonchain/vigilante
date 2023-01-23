package reporter

import (
	"fmt"
	"time"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

func (r *Reporter) Bootstrap(skipBlockSubscription bool) {
	var (
		btcLatestBlockHeight   uint64
		bbnBaseHeight          uint64
		bbnLatestBlockHeight   uint64
		consistencyCheckHeight uint64
		startSyncHeight        uint64
		ibs                    []*types.IndexedBlock
		err                    error
	)
	// ensure BTC has caught up with BBN header chain
	r.waitUntilBTCSync()

	// initialize cache with the latest blocks
	if err = r.initBTCCache(); err != nil {
		panic(err)
	}
	log.Debugf("BTC cache size: %d", r.btcCache.Size())

	// Subscribe new blocks right after initialising BTC cache, in order to ensure subscribed blocks and cached blocks do not have overlap.
	// Otherwise, if we subscribe too early, then they will have overlap, leading to duplicated header/ckpt submissions.
	if !skipBlockSubscription {
		r.btcClient.MustSubscribeBlocks()
	}

	// Initial consistency check: whether the `max(bbn_tip_height - confirmation_depth, bbn_base_height)`-th block is same
	// Find the latest block height in BBN header chain
	_, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		panic(err)
	}

	// Find the base height of BBN header chain
	_, bbnBaseHeight, err = r.babylonClient.QueryBaseHeader()
	if err != nil {
		panic(err)
	}

	// Find consistency check height
	if bbnLatestBlockHeight >= bbnBaseHeight+r.btcConfirmationDepth {
		consistencyCheckHeight = bbnLatestBlockHeight - r.btcConfirmationDepth + 1
	} else {
		consistencyCheckHeight = bbnBaseHeight
	}

	// make sure BBN headers are consistent with BTC
	r.checkHeaderConsistency(consistencyCheckHeight)

	// TODO: implement stalling check

	signer := r.babylonClient.MustGetAddr()

	// For each block higher than the k-deep block in BBN header chain, extract its header/ckpt and forward to BBN
	// If BBN has less than k blocks, sync from the 1st block in BBN,
	// since in this case the base header has passed the consistency check
	if bbnLatestBlockHeight >= bbnBaseHeight+r.btcConfirmationDepth {
		startSyncHeight = bbnLatestBlockHeight - r.btcConfirmationDepth + 1
	} else {
		startSyncHeight = bbnBaseHeight + 1
	}

	ibs, err = r.btcCache.GetLastBlocks(startSyncHeight)
	if err != nil {
		panic(err)
	}

	log.Infof("BTC height: %d. BTCLightclient height: %d. Start syncing from height %d.", btcLatestBlockHeight, bbnLatestBlockHeight, startSyncHeight)

	// extracts and submits headers for each block in ibs
	_, err = r.ProcessHeaders(signer, ibs)
	if err != nil {
		// this can happen when there are two contentious vigilantes
		log.Errorf("Failed to submit headers: %v", err)
		panic(err)
	}

	// trim cache to the latest k+w blocks on BTC (which are same as in BBN)
	maxEntries := r.btcConfirmationDepth + r.checkpointFinalizationTimeout
	if err = r.btcCache.Resize(maxEntries); err != nil {
		log.Errorf("Failed to resize BTC cache: %v", err)
		panic(err)
	}
	r.btcCache.Trim()

	log.Infof("Size of the BTC cache: %d", r.btcCache.Size())

	// fetch k+w blocks from cache and submit checkpoints
	ibs = r.btcCache.GetAllBlocks()
	_, _, err = r.ProcessCheckpoints(signer, ibs)
	if err != nil {
		log.Warnf("Failed to submit checkpoints: %v", err)
	}

	log.Info("Successfully finished bootstrapping")
}

// initBTCCache fetches the blocks since T-k-w in the BTC canonical chain
// where T is the height of the latest block in BBN header chain
func (r *Reporter) initBTCCache() error {
	var (
		err                  error
		bbnLatestBlockHeight uint64
		bbnBaseHeight        uint64
		baseHeight           uint64
		ibs                  []*types.IndexedBlock
	)

	r.btcCache, err = types.NewBTCCache(10000) // TODO: give an option to be unsized
	if err != nil {
		return err
	}

	// get T, i.e., total block count in BBN header chain
	_, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		return err
	}

	// Find the base height
	_, bbnBaseHeight, err = r.babylonClient.QueryBaseHeader()
	if err != nil {
		return err
	}

	// Fetch block since `baseHeight = T - k - w` from BTC, where
	// - T is total block count in BBN header chain
	// - k is btcConfirmationDepth of BBN
	// - w is checkpointFinalizationTimeout of BBN
	if bbnLatestBlockHeight > bbnBaseHeight+r.btcConfirmationDepth+r.checkpointFinalizationTimeout {
		baseHeight = bbnLatestBlockHeight - r.btcConfirmationDepth - r.checkpointFinalizationTimeout + 1
	} else {
		baseHeight = bbnBaseHeight
	}

	ibs, err = r.btcClient.FindTailBlocksByHeight(baseHeight)
	if err != nil {
		return err
	}

	if err = r.btcCache.Init(ibs); err != nil {
		return err
	}
	return nil
}

// waitUntilBTCSync waits for BTC to synchronize until BTC is no shorter than Babylon's BTC light client.
// It returns BTC last block hash, BTC last block height, and Babylon's base height.
func (r *Reporter) waitUntilBTCSync() {
	var (
		btcLatestBlockHash   *chainhash.Hash
		btcLatestBlockHeight uint64
		bbnLatestBlockHash   *chainhash.Hash
		bbnLatestBlockHeight uint64
		err                  error
	)

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
	if btcLatestBlockHeight == 0 || btcLatestBlockHeight < bbnLatestBlockHeight {
		log.Infof("BTC chain (length %d) falls behind BBN header chain (length %d), wait until BTC catches up", btcLatestBlockHeight, bbnLatestBlockHeight)

		// periodically check if BTC catches up with BBN.
		// When BTC catches up, break and continue the bootstrapping process
		ticker := time.NewTicker(5 * time.Second) // TODO: parameterise the polling interval
		for range ticker.C {
			_, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
			if err != nil {
				panic(err)
			}
			_, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
			if err != nil {
				panic(err)
			}
			if btcLatestBlockHeight > 0 && btcLatestBlockHeight >= bbnLatestBlockHeight {
				log.Infof("BTC chain (length %d) now catches up with BBN header chain (length %d), continue bootstrapping", btcLatestBlockHeight, bbnLatestBlockHeight)
				break
			}
			log.Infof("BTC chain (length %d) still falls behind BBN header chain (length %d), keep waiting", btcLatestBlockHeight, bbnLatestBlockHeight)
		}
	}
}

func (r *Reporter) checkHeaderConsistency(consistencyCheckHeight uint64) {
	var err error

	consistencyCheckBlock := r.btcCache.FindBlock(consistencyCheckHeight)
	if consistencyCheckBlock == nil {
		err = fmt.Errorf("cannot find the %d-th block of BBN header chain in BTC cache for initial consistency check", consistencyCheckHeight)
		panic(err)
	}
	consistencyCheckHash := consistencyCheckBlock.BlockHash()

	log.Debugf("block for consistency check: height %d, hash %v", consistencyCheckHeight, consistencyCheckHash)

	consistent, err := r.babylonClient.QueryContainsBlock(&consistencyCheckHash) // TODO: this API has error. Find out why
	if err != nil {
		panic(err)
	}
	if !consistent {
		err = fmt.Errorf("BTC main chain is inconsistent with BBN header chain: k-deep block in BBN header chain: %v", consistencyCheckHash)
		// TODO: produce and forward inconsistency evidence to BBN, make BBN panic
		panic(err)
	}
}
