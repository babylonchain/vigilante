package reporter

import (
	"fmt"
	"time"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func (r *Reporter) Init() {
	var (
		btcLatestBlockHash     *chainhash.Hash
		btcLatestBlockHeight   uint64
		bbnBaseHeight          uint64
		bbnLatestBlockHash     *chainhash.Hash
		bbnLatestBlockHeight   uint64
		consistencyCheckHeight uint64
		startSyncHeight        uint64
		tempBTCCache           *types.BTCCache
		err                    error
	)

	/* ensure BTC has catched up with BBN header chain */

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
	if btcLatestBlockHeight == 0 || uint64(btcLatestBlockHeight) < bbnLatestBlockHeight {
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
			if btcLatestBlockHeight > 0 && uint64(btcLatestBlockHeight) >= bbnLatestBlockHeight {
				log.Infof("BTC chain (length %d) now catches up with BBN header chain (length %d), continue bootstrapping", btcLatestBlockHeight, bbnLatestBlockHeight)
				break
			}
			log.Infof("BTC chain (length %d) still falls behind BBN header chain (length %d), keep waiting", btcLatestBlockHeight, bbnLatestBlockHeight)
		}
	}

	// update last block info for BTC client
	r.btcClient.LastBlockHash, r.btcClient.LastBlockHeight = btcLatestBlockHash, btcLatestBlockHeight

	// Now we have guaranteed that BTC is no shorter than BBN

	/* Initialize BTC Cache that includes current leading blocks of BTC, and subscribe to the forthcoming BTC blocks */

	// Download all blocks since height T-k-w from BTC, where
	// - T is total block count in BBN header chain
	// - k is btcConfirmationDepth of BBN
	// - w is checkpointFinalizationTimeout of BBN
	if tempBTCCache, err = r.initBTCCache(); err != nil {
		panic(err)
	}
	log.Debugf("BTC cache size: %d", tempBTCCache.Size())

	r.btcClient.SubscribeBlocks()

	/* Initial consistency check: whether the `max(bbn_tip_height - confirmation_depth, bbn_base_height)`-th block is same */

	// Find the block for consistency check
	// i.e., the block at height `max(bbn_tip_height - confirmation_depth, bbn_base_height)`
	if bbnLatestBlockHeight >= bbnBaseHeight+r.btcConfirmationDepth {
		consistencyCheckHeight = bbnLatestBlockHeight - r.btcConfirmationDepth + 1 // height of the k-deep block in BBN header chain
	} else {
		consistencyCheckHeight = bbnBaseHeight // height of the base header in BBN header chain
	}

	consistencyCheckBlock := tempBTCCache.FindBlock(consistencyCheckHeight)
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

	// TODO: initial stalling check

	/* help BBN to catch up with BTC */

	// For each block higher than the k-deep block in BBN header chain, extract its header/ckpt and forward to BBN
	// If BBN has less than k blocks, sync from the 1st block in BBN,
	// since in this case the base header has passed the consistency check
	if bbnLatestBlockHeight >= bbnBaseHeight+r.btcConfirmationDepth {
		startSyncHeight = bbnLatestBlockHeight - r.btcConfirmationDepth + 1
	} else {
		startSyncHeight = bbnBaseHeight + 1
	}

	ibs, err := tempBTCCache.GetLastBlocks(startSyncHeight)
	if err != nil {
		panic(err)
	}
	signer := r.babylonClient.MustGetAddr()

	log.Infof("BTC height: %d. BTCLightclient height: %d. Start syncing from height %d.", btcLatestBlockHeight, bbnLatestBlockHeight, startSyncHeight)

	// submit all headers in a single tx, with deduplication
	headers := []*wire.BlockHeader{}
	for _, ib := range ibs {
		headers = append(headers, ib.Header)
	}
	if err = r.submitHeaders(signer, headers); err != nil {
		log.Errorf("Failed to handle headers from Bitcoin: %v", err)
		panic(err)
	}

	// extract checkpoints and find matched checkpoints
	for _, ib := range ibs {
		log.Debugf("Block %v contains %d txs", ib.BlockHash(), len(ib.Txs))

		// extract checkpoints into the pool
		if r.extractCkpts(ib) == 0 {
			log.Infof("Block %v contains no tx with checkpoint segment, skip the matching attempt", ib.BlockHash())
			continue
		}

		// Find matched checkpoint segments and submit checkpoints
		if err = r.matchAndSubmitCkpts(signer); err != nil {
			log.Errorf("Failed to match and submit checkpoints to BBN: %v", err)
		}
	}

	/* initialise fixed-length BTC cache for reporter */

	// cut off tempBTCCache to the latest k+w blocks on BTC (which are same as in BBN)
	r.btcCache = tempBTCCache.TrimToSized(r.btcConfirmationDepth + r.checkpointFinalizationTimeout)

	log.Infof("Size of the BTC cache: %d", r.btcCache.Size())

	log.Info("Successfully finished bootstrapping")
}

// initBTCCache fetches the blocks since T-k-w in the BTC canonical chain
// where T is the height of the latest block in BBN header chain
func (r *Reporter) initBTCCache() (*types.BTCCache, error) {
	var (
		err                  error
		bbnLatestBlockHeight uint64
		bbnBaseHeight        uint64
		stopHeight           uint64
		ibs                  []*types.IndexedBlock
		btcCache             = types.NewBTCCache(10000) // TODO: give an option to be unsized
	)

	// get T, i.e., total block count in BBN header chain
	// TODO: now T is the height of BTC chain rather than BBN header chain
	_, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		return nil, err
	}

	// Find the base height
	_, bbnBaseHeight, err = r.babylonClient.QueryBaseHeader()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	if err = btcCache.Init(ibs); err != nil {
		return nil, err
	}
	return btcCache, nil
}
