package reporter

import (
	"fmt"
	"time"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

func (r *Reporter) Bootstrap() {
	var (
		btcLatestBlockHeight uint64
		bbnBaseHeight        uint64
		bbnLatestBlockHeight uint64
		startSyncHeight      uint64
		err                  error
	)

	// makes sure BBN header chain is not ahead of BTC
	r.btcClient.LastBlockHash, r.btcClient.LastBlockHeight, bbnBaseHeight = r.slowDownBBN()

	// initialize cache with the latest blocks
	if err = r.initBTCCache(); err != nil {
		panic(err)
	}
	log.Debugf("BTC cache size: %d", r.btcCache.Size())

	// make sure BBN headers are consistent with BTC
	r.checkHeaderConsistency(bbnLatestBlockHeight, bbnBaseHeight)

	// TODO: implement stalling check

	// send the latest BTC blocks to BBN
	if bbnLatestBlockHeight >= bbnBaseHeight+r.btcConfirmationDepth {
		startSyncHeight = bbnLatestBlockHeight - r.btcConfirmationDepth + 1
	} else {
		startSyncHeight = bbnBaseHeight + 1
	}

	ibs, err := r.btcCache.GetLastBlocks(startSyncHeight)
	if err != nil {
		panic(err)
	}
	signer := r.babylonClient.MustGetAddr()

	log.Infof("BTC height: %d. BTCLightclient height: %d. Start syncing from height %d.", btcLatestBlockHeight, bbnLatestBlockHeight, startSyncHeight)

	// extracts and submits headers for each block in ibs
	r.processHeaders(signer, ibs)

	// extracts and submits checkpoints for each block in ibs
	r.processCheckpoints(signer, ibs)

	// trim cache to the latest k+w blocks on BTC (which are same as in BBN)
	maxEntries := r.btcConfirmationDepth + r.checkpointFinalizationTimeout
	if err = r.btcCache.Trim(maxEntries); err != nil {
		log.Errorf("Failed to trim BTC cache: %v", err)
		panic(err)
	}

	log.Infof("Size of the BTC cache: %d", r.btcCache.Size())

	log.Info("Successfully finished bootstrapping")
}

// initBTCCache fetches the blocks since T-k-w in the BTC canonical chain
// where T is the height of the latest block in BBN header chain
func (r *Reporter) initBTCCache() error {
	var (
		err                  error
		bbnLatestBlockHeight uint64
		bbnBaseHeight        uint64
		stopHeight           uint64
		ibs                  []*types.IndexedBlock
	)

	r.btcCache, err = types.NewBTCCache(10000) // TODO: give an option to be unsized
	if err != nil {
		return err
	}

	// get T, i.e., total block count in BBN header chain
	// TODO: now T is the height of BTC chain rather than BBN header chain
	_, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		return err
	}

	// Find the base height
	_, bbnBaseHeight, err = r.babylonClient.QueryBaseHeader()
	if err != nil {
		return err
	}

	// Fetch block since `stopHeight = T - k - w` from BTC, where
	// - T is total block count in BBN header chain
	// - k is btcConfirmationDepth of BBN
	// - w is checkpointFinalizationTimeout of BBN
	if bbnLatestBlockHeight > bbnBaseHeight+r.btcConfirmationDepth+r.checkpointFinalizationTimeout {
		stopHeight = bbnLatestBlockHeight - r.btcConfirmationDepth - r.checkpointFinalizationTimeout + 1
	} else {
		stopHeight = bbnBaseHeight
	}

	ibs, err = r.btcClient.GetLastBlocks(stopHeight)
	if err != nil {
		return err
	}

	if err = r.btcCache.Init(ibs); err != nil {
		return err
	}
	return nil
}

func (r *Reporter) slowDownBBN() (*chainhash.Hash, uint64, uint64) {
	var (
		btcLatestBlockHash   *chainhash.Hash
		btcLatestBlockHeight uint64
		bbnBaseHeight        uint64
		bbnLatestBlockHash   *chainhash.Hash
		bbnLatestBlockHeight uint64
		err                  error
	)

	// Find the base height of BTCLightclient
	_, bbnBaseHeight, err = r.babylonClient.QueryBaseHeader()
	if err != nil {
		panic(err)
	}

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
			btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
			if err != nil {
				panic(err)
			}
			bbnLatestBlockHash, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
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

	return btcLatestBlockHash, btcLatestBlockHeight, bbnBaseHeight

}

func (r *Reporter) checkHeaderConsistency(bbnLatestBlockHeight, bbnBaseHeight uint64) {
	var (
		consistencyCheckHeight uint64
		err                    error
	)
	if bbnLatestBlockHeight >= bbnBaseHeight+r.btcConfirmationDepth {
		consistencyCheckHeight = bbnLatestBlockHeight - r.btcConfirmationDepth + 1 // height of the k-deep block in BBN header chain
	} else {
		consistencyCheckHeight = bbnBaseHeight // height of the base header in BBN header chain
	}

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
