package reporter

import (
	"fmt"

	"github.com/babylonchain/vigilante/types"
)

// blockEventHandler handles connected and disconnected blocks from the BTC client.
func (r *Reporter) blockEventHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case event, open := <-r.btcClient.BlockEventChan():
			if !open {
				r.logger.Errorf("Block event channel is closed")
				return // channel closed
			}

			var errorRequiringBootstrap error
			if event.EventType == types.BlockConnected {
				errorRequiringBootstrap = r.handleConnectedBlocks(event)
			} else if event.EventType == types.BlockDisconnected {
				errorRequiringBootstrap = r.handleDisconnectedBlocks(event)
			}

			if errorRequiringBootstrap != nil {
				r.logger.Warnf("Due to error in event processing: %v, bootstrap process need to be restarted", errorRequiringBootstrap)
				r.bootstrapWithRetries(true)
			}

		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

// handleConnectedBlocks handles connected blocks from the BTC client.
func (r *Reporter) handleConnectedBlocks(event *types.BlockEvent) error {
	// if the header is too early, ignore it
	// NOTE: this might happen when bootstrapping is triggered after the reporter
	// has subscribed to the BTC blocks
	firstCacheBlock := r.btcCache.First()
	if firstCacheBlock == nil {
		return fmt.Errorf("cache is empty, restart bootstrap process")
	}
	if event.Height < firstCacheBlock.Height {
		r.logger.Debugf(
			"the connecting block (height: %d, hash: %s) is too early, skipping the block",
			event.Height,
			event.Header.BlockHash().String(),
		)
		return nil
	}

	// if the received header is within the cache's region, then this means the events have
	// an overlap with the cache. Then, perform a consistency check. If the block is duplicated,
	// then ignore the block, otherwise there is an inconsistency and redo bootstrap
	// NOTE: this might happen when bootstrapping is triggered after the reporter
	// has subscribed to the BTC blocks
	if b := r.btcCache.FindBlock(uint64(event.Height)); b != nil {
		if b.BlockHash() == event.Header.BlockHash() {
			r.logger.Debugf(
				"the connecting block (height: %d, hash: %s) is known to cache, skipping the block",
				b.Height,
				b.BlockHash().String(),
			)
			return nil
		}
		return fmt.Errorf(
			"the connecting block (height: %d, hash: %s) is different from the header (height: %d, hash: %s) at the same height in cache",
			event.Height,
			event.Header.BlockHash().String(),
			b.Height,
			b.BlockHash().String(),
		)
	}

	// get the block from hash
	blockHash := event.Header.BlockHash()
	ib, mBlock, err := r.btcClient.GetBlockByHash(&blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block %v with number %d ,from BTC client: %w", blockHash, event.Height, err)
	}

	// if the parent of the block is not the tip of the cache, then the cache is not up-to-date,
	// and we might have missed some blocks. In this case, restart the bootstrap process.
	parentHash := mBlock.Header.PrevBlock
	cacheTip := r.btcCache.Tip() // NOTE: cache is guaranteed to be non-empty at this stage
	if parentHash != cacheTip.BlockHash() {
		return fmt.Errorf("cache (tip %d) is not up-to-date while connecting block %d, restart bootstrap process", cacheTip.Height, ib.Height)
	}

	// otherwise, add the block to the cache
	r.btcCache.Add(ib)

	var headersToProcess []*types.IndexedBlock

	if r.reorgList.size() > 0 {
		// we are in the middle of reorg, we need to check whether we already have all blocks of better chain
		// as reorgs in btc nodes happen only when better chain is available.
		// 1. First we get oldest header from our reorg branch
		// 2. Then we get all headers from our cache starting the height of the oldest header of new branch
		// 3. then we calculate if work on new branch starting from the first reorged height is larger
		// than removed branch work.
		oldestBlockFromOldBranch := r.reorgList.getLastRemovedBlock()
		currentBranch, err := r.btcCache.GetLastBlocks(oldestBlockFromOldBranch.height)
		if err != nil {
			panic(fmt.Errorf("failed to get block from cache after reorg: %w", err))
		}

		currentBranchWork := calculateBranchWork(currentBranch)

		// if current branch is better than reorg branch, we can submit headers and clear reorg list
		if currentBranchWork.GT(r.reorgList.removedBranchWork()) {
			r.logger.Debugf("Current branch is better than reorg branch. Length of current branch: %d, work of branch: %s", len(currentBranch), currentBranchWork)
			headersToProcess = append(headersToProcess, currentBranch...)
			r.reorgList.clear()
		}
	} else {
		headersToProcess = append(headersToProcess, ib)
	}

	if len(headersToProcess) == 0 {
		r.logger.Debug("No new headers to submit to Babylon")
		return nil
	}

	// extracts and submits headers for each blocks in ibs
	signer := r.babylonClient.MustGetAddr()
	_, err = r.ProcessHeaders(signer, headersToProcess)
	if err != nil {
		r.logger.Warnf("Failed to submit header: %v", err)
	}

	// extracts and submits checkpoints for each blocks in ibs
	_, _, err = r.ProcessCheckpoints(signer, headersToProcess)
	if err != nil {
		r.logger.Warnf("Failed to submit checkpoint: %v", err)
	}
	return nil
}

// handleDisconnectedBlocks handles disconnected blocks from the BTC client.
func (r *Reporter) handleDisconnectedBlocks(event *types.BlockEvent) error {
	// get cache tip
	cacheTip := r.btcCache.Tip()
	if cacheTip == nil {
		return fmt.Errorf("cache is empty, restart bootstrap process")
	}

	// if the block to be disconnected is not the tip of the cache, then the cache is not up-to-date,
	if event.Header.BlockHash() != cacheTip.BlockHash() {
		return fmt.Errorf("cache is not up-to-date while disconnecting block, restart bootstrap process")
	}

	// at this point, the block to be disconnected is the tip of the cache so we can
	// add it to our reorg list
	r.reorgList.addRemovedBlock(
		uint64(cacheTip.Height),
		cacheTip.Header,
	)

	// otherwise, remove the block from the cache
	if err := r.btcCache.RemoveLast(); err != nil {
		r.logger.Warnf("Failed to remove last block from cache: %v, restart bootstrap process", err)
		panic(err)
	}

	return nil
}
