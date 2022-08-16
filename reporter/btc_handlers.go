package reporter

func (r *Reporter) handleBTCTxs() {
	for tx := range r.btcClient.TxChan {
		// TODO: decode to objects
		entry1, entry2 := filterTx(tx)
		// TODO: check if the filtered entry can assemble with an existing entry to a new valid ckpt, and forward to BTCCheckpoint module
		if entry1 != nil {
			log.Info("Found a first half %v from Tx %v", entry1, tx.TxHash())
		} else if entry2 != nil {
			log.Info("Found a second half %v from Tx %v", entry1, tx.TxHash())
		} else {
			continue
		}
	}
}

func (r *Reporter) handleBTCHeaders() {
	for header := range r.btcClient.HeaderChan {
		log.Info("Received a new block %v", header.BlockHash())
		// TODO: forward header to BTCLightclient module
	}
}
