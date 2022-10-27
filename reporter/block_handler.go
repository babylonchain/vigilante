package reporter

import (
	"github.com/babylonchain/vigilante/types"
)

func (r *Reporter) blockEventHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case event := <-r.btcClient.BlockEventChan:
			if event.EventType == types.BlockConnected {
				r.handleConnectedBlocks(event)
			} else if event.EventType == types.BlockDisconnected {
				r.handleDisconnectedBlocks(event)
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (r *Reporter) handleConnectedBlocks(event *types.BlockEvent) {
	signer := r.babylonClient.MustGetAddr()

	// get the block from hash
	blockHash := event.Header.BlockHash()
	ib, mBlock, err := r.btcClient.GetBlockByHash(&blockHash)
	if err != nil {
		log.Errorf("Failed to get block %v from BTC client: %v", blockHash, err)
		panic(err)
	}

	// get cache tip
	cacheTip := r.btcCache.Tip()
	if cacheTip == nil {
		log.Warnf("Cache is empty, restart bootstrap process")
		r.Bootstrap(true)
		return
	}

	parentHash := mBlock.Header.PrevBlock

	// if the parent of the block is not the tip of the cache, then the cache is not up-to-date,
	// and we might have missed some blocks. In this case, restart the bootstrap process.
	if parentHash != cacheTip.BlockHash() {
		log.Warnf("Cache is not up-to-date, restart bootstrap process")
		r.Bootstrap(true)
		return
	}

	// otherwise, add the block to the cache
	r.btcCache.Add(ib)

	// extracts and submits headers for each block in ibs
	r.processHeaders(signer, []*types.IndexedBlock{ib})

	// extracts and submits checkpoints for each block in ibs
	r.processCheckpoints(signer, []*types.IndexedBlock{ib})
}

func (r *Reporter) handleDisconnectedBlocks(event *types.BlockEvent) {
	// get cache tip
	cacheTip := r.btcCache.Tip()
	if cacheTip == nil {
		log.Warnf("Cache is empty, restart bootstrap process")
		r.Bootstrap(true)
		return
	}

	// if the block to be disconnected is not the tip of the cache, then the cache is not up-to-date,
	if event.Header.BlockHash() != cacheTip.BlockHash() {
		log.Warnf("Cache is not up-to-date, restart bootstrap process")
		r.Bootstrap(true)
		return
	}

	// otherwise, remove the block from the cache
	if err := r.btcCache.RemoveLast(); err != nil {
		log.Warnf("Failed to remove last block from cache: %v, restart bootstrap process", err)
		r.Bootstrap(true)
		panic(err)
	}
}
