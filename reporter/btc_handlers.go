package reporter

import "github.com/babylonchain/vigilante/btcclient"

func (r *Reporter) indexedBlockHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case ib := <-r.btcClient.IndexedBlockChan:
			log.Infof("Start handling block %v from BTC client", ib.BlockHash())
			// dispatch the indexed block to the handler
			r.handleIndexedBlock(ib)
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (r *Reporter) handleIndexedBlock(ib *btcclient.IndexedBlock) {
	// handle header
	// TODO: forward this header to BTCLightclient module
	header := ib.Header
	log.Debugf("Received a new block %v", header.BlockHash())

	// handle each tx
	for _, tx := range ib.Txs {
		// TODO: decode to objects
		entry1, entry2 := filterTx(tx.MsgTx())
		if entry1 != nil {
			log.Infof("Found a first half %v from Tx %v", entry1, tx.Hash())
		} else if entry2 != nil {
			log.Infof("Found a second half %v from Tx %v", entry1, tx.Hash())
		} else {
			continue
		}
		// TODO: check if the filtered entry can assemble with an existing entry to a new valid ckpt
		// TODO: upon a newly assembled checkpoint, forward it to BTCCheckpoint module
	}
}
