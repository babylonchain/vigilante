package reporter

func (r *Reporter) handleBTCTxs() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case tx := <-r.btcClient.TxChan:
			// TODO: decode to objects
			entry1, entry2 := filterTx(tx)
			if entry1 != nil {
				log.Infof("Found a first half %v from Tx %v", entry1, tx.TxHash())
			} else if entry2 != nil {
				log.Infof("Found a second half %v from Tx %v", entry1, tx.TxHash())
			} else {
				continue
			}
			// TODO: check if the filtered entry can assemble with an existing entry to a new valid ckpt
			// TODO: upon a newly assembled checkpoint, forward it to BTCCheckpoint module
		case <-quit:
			return
		}
	}
}

func (r *Reporter) handleBTCHeaders() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case header := <-r.btcClient.HeaderChan:
			log.Infof("Received a new block %v", header.BlockHash())
			// TODO: forward this header to BTCLightclient module
		case <-quit:
			return
		}
	}
}
