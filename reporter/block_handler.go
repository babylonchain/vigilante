package reporter

import (
	"errors"
	"github.com/babylonchain/vigilante/types"
)

func (r *Reporter) blockEventHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	signer := r.babylonClient.MustGetAddr()
	for {
		select {
		case event := <-r.btcClient.BlockEventChan:
			if event.EventType == types.BlockConnected {
				// get the block from hash
				blockHash := event.Header.BlockHash()
				ib, mBlock, err := r.btcClient.GetBlockByHash(&blockHash)
				if err != nil {
					log.Errorf("Failed to get block %v from BTC client: %v", blockHash, err)
					panic(err)
				}

				// get cache tip
				cacheTip, err := r.btcCache.Tip()
				if err != nil {
					if errors.Is(err, types.ErrEmptyCache) {
						log.Errorf("Cache is empty, restart bootstrap process")
						r.Bootstrap()
						return
					}

					log.Errorf("Failed to get cache tip: %v", err)
					panic(err)
				}

				parentHash := mBlock.Header.PrevBlock

				// if the parent of the block is not the tip of the cache, then the cache is not up-to-date,
				// and we might have missed some blocks. In this case, restart the bootstrap process.
				if parentHash != cacheTip.BlockHash() {
					r.Bootstrap()
				} else {
					// otherwise, add the block to the cache
					if err := r.btcCache.Add(ib); err != nil {
						log.Errorf("Failed to add block %v to cache: %v", blockHash, err)
						panic(err)
					}

					// extracts and submits headers for each block in ibs
					r.processHeaders(signer, []*types.IndexedBlock{ib})

					// extracts and submits checkpoints for each block in ibs
					r.processCheckpoints(signer, []*types.IndexedBlock{ib})
				}
			} else if event.EventType == types.BlockDisconnected {
				// get cache tip
				cacheTip, err := r.btcCache.Tip()
				if err != nil {
					if errors.Is(err, types.ErrEmptyCache) {
						log.Errorf("Cache is empty, restart bootstrap process")
						r.Bootstrap()
						return
					}

					log.Errorf("Failed to get cache tip: %v", err)
					panic(err)
				}

				// if the block to be disconnected is not the tip of the cache, then the cache is not up-to-date,
				if event.Header.BlockHash() != cacheTip.BlockHash() {
					r.Bootstrap()
				} else {
					// otherwise, remove the block from the cache
					if err := r.btcCache.RemoveLast(); err != nil {
						log.Errorf("Failed to remove last block from cache: %v", err)
						panic(err)
					}
				}
			}

		case <-quit:
			// We have been asked to stop
			return
		}
	}
}
