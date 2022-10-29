package reporter

import (
	"encoding/hex"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// blockEventHandler handles connected and disconnected blocks from the BTC client.
func (r *Reporter) blockEventHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case event, open := <-r.btcClient.BlockEventChan():
			if !open {
				log.Errorf("Block event channel is closed")
				return // channel closed
			}

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

// zmqSequenceMessageHandler handles sequence messages from the ZMQ sequence socket and sends event
// to the block event channel.
func (r *Reporter) zmqSequenceMessageHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case msg, open := <-r.btcClient.ZmqSequenceMsgChan:
			if !open {
				log.Errorf("ZMQ sequence message channel is closed")
				return // channel closed
			}

			blockHashStr := hex.EncodeToString(msg.Hash[:])
			blockHash, err := chainhash.NewHashFromStr(blockHashStr)
			if err != nil {
				log.Errorf("Failed to parse block hash %v: %v", blockHashStr, err)
				panic(err)
			}

			ib, _, err := r.btcClient.GetBlockByHash(blockHash)
			if err != nil {
				log.Errorf("Failed to get block %v from BTC client: %v", blockHash, err)
				panic(err)
			}

			if msg.Event == types.BlockConnected {
				r.btcClient.BlockEventChan <- types.NewBlockEvent(types.BlockConnected, ib.Height, ib.Header)
			} else if msg.Event == types.BlockDisconnected {
				r.btcClient.BlockEventChan <- types.NewBlockEvent(types.BlockDisconnected, ib.Height, ib.Header)
			}

			log.Infof("Received ZMQ sequence message: %v", msg)

		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

// handleConnectedBlocks handles connected blocks from the BTC client.
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
	r.ProcessHeaders(signer, []*types.IndexedBlock{ib})

	// extracts and submits checkpoints for each block in ibs
	r.ProcessCheckpoints(signer, []*types.IndexedBlock{ib})
}

// handleDisconnectedBlocks handles disconnected blocks from the BTC client.
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
